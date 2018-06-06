CREATE TABLE hardware (
	id UUID UNIQUE
	, inserted_at TIMESTAMPTZ
	, deleted_at TIMESTAMPTZ
	, data JSONB
);

CREATE INDEX idx_id ON hardware (id);
CREATE INDEX idx_deleted_at ON hardware (deleted_at NULLS FIRST);
CREATE INDEX idxgin_type ON hardware USING GIN (data JSONB_PATH_OPS);
