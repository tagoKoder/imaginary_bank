// micro\ledger\cmd\ledger\main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
	accountsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/accounts/v1"
	"github.com/tagoKoder/ledger/internal/application/service"
	"github.com/tagoKoder/ledger/internal/infra/config"
	"github.com/tagoKoder/ledger/internal/infra/in/grpc/handler"
	"github.com/tagoKoder/ledger/internal/infra/in/grpc/middleware"
	"github.com/tagoKoder/ledger/internal/infra/out/audit"
	"github.com/tagoKoder/ledger/internal/infra/out/gateway"
	"github.com/tagoKoder/ledger/internal/infra/out/messaging"
	gormdb "github.com/tagoKoder/ledger/internal/infra/out/persistence/gorm"
	gormuow "github.com/tagoKoder/ledger/internal/infra/out/persistence/gorm/uow"
	"github.com/tagoKoder/ledger/internal/infra/security/authz"
	"github.com/tagoKoder/ledger/internal/infra/security/avp"

	ledgerpb "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	securitygrpc "github.com/tagoKoder/ledger/internal/infra/security/grpc"
	jwtvalidator "github.com/tagoKoder/ledger/internal/infra/security/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()

	if cfg.DBWriteDSN == "" {
		log.Fatal("DB_DSN (or DB_WRITE_DSN) is required")
	}

	// DB
	writeDB, err := gormdb.New(cfg.DBWriteDSN)
	if err != nil {
		log.Fatalf("db write: %v", err)
	}
	log.Println("database connected at ", cfg.DBWriteDSN)

	readDB := writeDB
	if cfg.DBReadDSN != "" {
		readDB, err = gormdb.New(cfg.DBReadDSN)
		if err != nil {
			log.Fatalf("db read: %v", err)
		}
	}

	// UoW (infra -> implements application.UnitOfWork)
	uow := gormuow.NewGormUnitOfWork(readDB, writeDB)

	// AWS SDK (EventBridge  AVP)
	awsCfg, err := awscfg.LoadDefaultConfig(
		context.Background(),
		awscfg.WithRegion(cfg.AWSRegion),
	)
	if err != nil {
		log.Fatalf("aws cfg: %v", err)
	}

	// EventBridge client  publisher
	eb := eventbridge.NewFromConfig(awsCfg)
	pub := messaging.NewEventBridgePublisher(eb, cfg.DomainEventBusName, "ledger-payments")

	//vp := verifiedpermissions.NewFromConfig(awsCfg)

	// AuditPort: EventBridge best-effort (fallback a logs interno)
	auditPort := audit.NewEventBridgeAudit(
		eb,
		cfg.AuditEventBusName,
		cfg.AuditSource,
		cfg.AuditDetailType,
		cfg.ServiceName,
		cfg.AppEnv,
	)

	// Accounts gRPC client (internal)
	accountsConn, err := dialAccountsGRPC(cfg)
	if err != nil {
		log.Printf("Account address: %v", cfg.AccountsGrpcTarget)
		log.Printf("accounts grpc dial: %v", err)
	}
	accountsClient := accountsv1.NewInternalAccountsServiceClient(accountsConn)
	accountsGW := gateway.NewAccountsGRPCGateway(
		accountsClient,
		cfg.AccountsGrpcTimeout,
		cfg.AccountsInternalTok,
		cfg.InternalTokenHeader,
	)

	// Application services
	paySvc := service.NewPaymentService(uow, accountsGW, auditPort)
	ledSvc := service.NewLedgerAppService(uow, accountsGW, auditPort)

	// gRPC handlers
	payH := handler.NewPaymentsHandler(paySvc, paySvc)
	ledH := handler.NewLedgerHandler(ledSvc, ledSvc, ledSvc)

	// Outbox worker
	outboxWorker := messaging.NewOutboxWorker(uow, pub, cfg.OutboxBatchSize, cfg.OutboxPollInterval)
	outboxWorker.Start()

	vp := verifiedpermissions.NewFromConfig(awsCfg)

	jv := jwtvalidator.New(cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTJWKSURL, &cfg)
	avpClient := avp.New(vp, cfg.AVPPolicyStoreID)

	reg := authz.NewRegistry()
	tpl := authz.NewTemplates(uow)

	authzItc := securitygrpc.NewAuthzInterceptor(
		jv,
		avpClient,
		reg,
		tpl,
		auditPort,
		cfg.HashSaltIP,
		cfg.HashSaltUA,
	)

	// gRPC server
	lis, err := net.Listen("tcp", ":"+cfg.GrpcPort)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.NewUnaryErrorInterceptor(cfg.AppEnv),
			authzItc.Unary(),
		),
	)

	ledgerpb.RegisterPaymentsServiceServer(grpcSrv, payH)
	ledgerpb.RegisterLedgerServiceServer(grpcSrv, ledH)
	if cfg.AppEnv != "local" {
		reflection.Register(grpcSrv)
	}

	// graceful shutdown
	go func() {
		log.Printf("ledger-payments gRPC listening on %s env=%s", cfg.GrpcPort, cfg.AppEnv)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()

	waitForShutdown(func() {
		outboxWorker.Stop()
		stopped := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			grpcSrv.Stop()
		}
		_ = lis.Close()
		log.Println("shutdown complete")
	})
}

func waitForShutdown(fn func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fn()
}

func dialAccountsGRPC(cfg config.Config) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	var tc credentials.TransportCredentials
	if cfg.AccountsGrpcInsecure {
		tc = insecure.NewCredentials()
	} else {
		if cfg.AccountsGrpcTLSCaFile == "" {
			return nil, fmt.Errorf("ACCOUNTS_GRPC_TLS_CA_FILE is required when ACCOUNTS_GRPC_INSECURE=false")
		}
		creds, err := credentials.NewClientTLSFromFile(cfg.AccountsGrpcTLSCaFile, cfg.AccountsGrpcTLSServerName)
		if err != nil {
			return nil, err
		}
		tc = creds
	}

	return grpc.DialContext(
		ctx,
		cfg.AccountsGrpcTarget,
		grpc.WithTransportCredentials(tc),
		grpc.WithBlock(),
	)
}
