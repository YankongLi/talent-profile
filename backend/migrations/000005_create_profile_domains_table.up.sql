CREATE TABLE profile_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT true,
    redirect_to_slug TEXT,
    redirect_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT profile_domains_slug_valid CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,28}[a-z0-9]$'),
    CONSTRAINT profile_domains_slug_not_reserved CHECK (
        slug NOT IN ('admin', 'api', 'app', 'assets', 'cdn', 'login', 'mail', 'static', 'www')
    ),
    CONSTRAINT profile_domains_redirect_slug_valid CHECK (
        redirect_to_slug IS NULL OR redirect_to_slug ~ '^[a-z0-9][a-z0-9-]{1,28}[a-z0-9]$'
    )
);

CREATE UNIQUE INDEX profile_domains_slug_unique_idx
    ON profile_domains (slug);

CREATE INDEX profile_domains_profile_idx
    ON profile_domains (profile_id, is_primary);

CREATE TRIGGER profile_domains_set_updated_at
    BEFORE UPDATE ON profile_domains
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
