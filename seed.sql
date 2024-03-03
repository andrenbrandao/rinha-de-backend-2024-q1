DROP TABLE IF EXISTS accounts;

CREATE TABLE IF NOT EXISTS accounts (
  id SERIAL NOT NULL,
  name VARCHAR NOT NULL,
  balance INTEGER DEFAULT 0 NOT NULL,
  balance_limit INTEGER DEFAULT 0 NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

INSERT INTO accounts
  (name, balance_limit)
VALUES
  ('John Doe', 1000*100),
  ('Jane Doe', 800*100),
  ('Jack Sparrow', 10000*100),
  ('Bruce Wayne', 100000*100),
  ('Scarlett Johansson', 5000*100);

-- Create transactions
DROP TABLE IF EXISTS transactions;

CREATE TABLE IF NOT EXISTS transactions (
  id SERIAL NOT NULL,
  amount INTEGER NOT NULL,
  description VARCHAR NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);
