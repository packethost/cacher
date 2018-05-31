CREATE TABLE hardware (
	id UUID UNIQUE
	, inserted_at TIMESTAMPTZ
	, data JSONB
);

CREATE INDEX idx_id ON hardware (id);
CREATE INDEX idxgin_type ON hardware USING GIN (data JSONB_PATH_OPS);
