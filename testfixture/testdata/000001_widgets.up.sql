BEGIN;

CREATE TABLE IF NOT EXISTS fixtures.widgets (
  id    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name  text NOT NULL,
  ctime timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS widgets_name ON fixtures.widgets (name);

COMMIT;
