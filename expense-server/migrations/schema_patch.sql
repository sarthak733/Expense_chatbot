-- 0. Create core tables if they do not exist
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- 1. Create categories table
CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE, -- NULL means system/default category
    name VARCHAR(100) NOT NULL,
    color VARCHAR(7) DEFAULT '#808080', -- Hex code, e.g., #FF5733
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_user_category_name UNIQUE(user_id, name)
);

-- Seed default categories if they do not exist
INSERT INTO categories (user_id, name, color) VALUES
(NULL, 'Food', '#FF5733'),
(NULL, 'Transport', '#3357FF'),
(NULL, 'Utilities', '#33FF57'),
(NULL, 'Entertainment', '#F033FF'),
(NULL, 'Shopping', '#FF33A6'),
(NULL, 'Others', '#808080')
ON CONFLICT (user_id, name) DO NOTHING;

-- 2. Modify expenses table to include category_id
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL;

-- Backfill category_id in existing expenses
-- First match system categories
UPDATE expenses e
SET category_id = c.id
FROM categories c
WHERE c.user_id IS NULL AND LOWER(e.category) = LOWER(c.name) AND e.category_id IS NULL;

-- For any remaining expenses that don't match system categories, create custom categories for the respective users
INSERT INTO categories (user_id, name, color)
SELECT DISTINCT e.user_id, e.category, '#808080'
FROM expenses e
WHERE e.category_id IS NULL AND e.category <> ''
ON CONFLICT (user_id, name) DO NOTHING;

-- Now update the rest of the expenses to point to the newly created user categories
UPDATE expenses e
SET category_id = c.id
FROM categories c
WHERE e.user_id = c.user_id AND LOWER(e.category) = LOWER(c.name) AND e.category_id IS NULL;

-- 3. Create budgets table
CREATE TABLE IF NOT EXISTS budgets (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id) ON DELETE CASCADE, -- NULL means overall budget
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    period VARCHAR(20) NOT NULL DEFAULT 'monthly', -- 'weekly', 'monthly', 'yearly'
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT check_dates CHECK (end_date >= start_date)
);

-- 4. Create recurring_expenses table
CREATE TABLE IF NOT EXISTS recurring_expenses (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    interval VARCHAR(20) NOT NULL, -- 'daily', 'weekly', 'monthly', 'yearly'
    next_run_date DATE NOT NULL,
    last_run_date DATE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
