CREATE TABLE resumes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    content_hash TEXT NOT NULL,
    extracted_text_encrypted BYTEA,
    parse_status TEXT NOT NULL DEFAULT 'uploaded',
    parse_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT resumes_storage_key_not_blank CHECK (btrim(storage_key) <> ''),
    CONSTRAINT resumes_original_filename_not_blank CHECK (btrim(original_filename) <> ''),
    CONSTRAINT resumes_mime_type_valid CHECK (
        mime_type IN (
            'application/pdf',
            'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
        )
    ),
    CONSTRAINT resumes_file_size_positive CHECK (file_size > 0),
    CONSTRAINT resumes_content_hash_sha256 CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT resumes_parse_status_valid CHECK (
        parse_status IN ('uploaded', 'parsing', 'parsed', 'failed')
    )
);

CREATE INDEX resumes_user_created_at_idx
    ON resumes (user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX resumes_content_hash_idx
    ON resumes (content_hash)
    WHERE deleted_at IS NULL;
