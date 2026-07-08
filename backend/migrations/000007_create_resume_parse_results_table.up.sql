CREATE TABLE resume_parse_results (
    resume_id UUID PRIMARY KEY REFERENCES resumes(id) ON DELETE CASCADE,
    extracted_text_encrypted BYTEA NOT NULL,
    redacted_text TEXT NOT NULL,
    sensitive_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT resume_parse_results_redacted_text_not_blank CHECK (btrim(redacted_text) <> ''),
    CONSTRAINT resume_parse_results_sensitive_fields_array CHECK (
        jsonb_typeof(sensitive_fields_json) = 'array'
    )
);

CREATE TRIGGER resume_parse_results_set_updated_at
    BEFORE UPDATE ON resume_parse_results
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
