-- FASE 1: reglas estructurales. NO modifica datos existentes automáticamente.
-- Si una comprobación falla, revisar manualmente los datos antes de continuar.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM accounts GROUP BY customer_id HAVING count(*) > 1) THEN
    RAISE EXCEPTION 'F1: existen clientes con varias cuentas; conciliar antes de aplicar UNIQUE';
  END IF;
  IF EXISTS (SELECT 1 FROM account_opening_bonus_grants WHERE account_id IS NULL) THEN
    RAISE EXCEPTION 'F1: existen bonos pendientes sin account_id; reconciliar antes de migrar';
  END IF;
  IF EXISTS (
    SELECT 1 FROM account_opening_bonus_grants g
    LEFT JOIN accounts a ON a.id = g.account_id
    WHERE a.id IS NULL
  ) THEN
    RAISE EXCEPTION 'F1: existen bonos asociados a cuentas inexistentes';
  END IF;
  IF EXISTS (
    SELECT 1 FROM account_opening_bonus_grants g JOIN accounts a ON a.id=g.account_id
    GROUP BY a.customer_id HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'F1: existen varios bonos para el mismo cliente; reconciliar';
  END IF;
END $$;

ALTER TABLE account_opening_bonus_grants
  ADD COLUMN customer_id uuid,
  ADD COLUMN benefit_code varchar(64) NOT NULL DEFAULT 'REGISTRATION_BONUS';

UPDATE account_opening_bonus_grants g
SET customer_id = a.customer_id
FROM accounts a
WHERE a.id = g.account_id;

ALTER TABLE account_opening_bonus_grants
  ALTER COLUMN customer_id SET NOT NULL,
  ALTER COLUMN idempotency_key SET NOT NULL,
  ADD CONSTRAINT fk_bonus_customer FOREIGN KEY (customer_id) REFERENCES customers(id),
  ADD CONSTRAINT fk_bonus_account FOREIGN KEY (account_id) REFERENCES accounts(id),
  ADD CONSTRAINT ck_bonus_status CHECK (status IN ('PENDING','COMPLETED')),
  ADD CONSTRAINT ck_bonus_completion CHECK (
     (status = 'PENDING' AND journal_id IS NULL)
     OR (status = 'COMPLETED' AND account_id IS NOT NULL AND journal_id IS NOT NULL)
  ),
  ADD CONSTRAINT ck_bonus_amount CHECK (amount > 0),
  ADD CONSTRAINT ck_bonus_currency CHECK (currency = 'USD'),
  ADD CONSTRAINT ck_bonus_campaign CHECK (benefit_code = 'REGISTRATION_BONUS' AND amount = 50.00);

CREATE UNIQUE INDEX uq_accounts_one_per_customer ON accounts(customer_id);
CREATE UNIQUE INDEX uq_bonus_one_per_customer_benefit
  ON account_opening_bonus_grants(customer_id, benefit_code);
CREATE UNIQUE INDEX uq_bonus_journal_id
  ON account_opening_bonus_grants(journal_id) WHERE journal_id IS NOT NULL;

-- Evita asociar una cuenta de otro cliente al bono (FK compuesta).
ALTER TABLE accounts ADD CONSTRAINT uq_accounts_id_customer UNIQUE (id, customer_id);
ALTER TABLE account_opening_bonus_grants
  ADD CONSTRAINT fk_bonus_account_same_customer
  FOREIGN KEY (account_id, customer_id) REFERENCES accounts(id, customer_id);
