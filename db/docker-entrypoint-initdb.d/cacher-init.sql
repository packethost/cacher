SET ROLE postgres;

CREATE TABLE IF NOT EXISTS hardware (
	id UUID UNIQUE
	, inserted_at TIMESTAMPTZ
	, deleted_at TIMESTAMPTZ
	, data JSONB
);

CREATE INDEX IF NOT EXISTS idx_id ON hardware (id);
CREATE INDEX IF NOT EXISTS idx_deleted_at ON hardware (deleted_at NULLS FIRST);
CREATE INDEX IF NOT EXISTS idxgin_type ON hardware USING GIN (data JSONB_PATH_OPS);
