-- Add currency column to expenses table
ALTER TABLE "expenses" ADD COLUMN IF NOT EXISTS "currency" character varying NOT NULL DEFAULT 'USD';

-- Add currency column to budgets table
ALTER TABLE "budgets" ADD COLUMN IF NOT EXISTS "currency" character varying NOT NULL DEFAULT 'USD';

-- Add currency column to recurring_expenses table
ALTER TABLE "recurring_expenses" ADD COLUMN IF NOT EXISTS "currency" character varying NOT NULL DEFAULT 'USD';
