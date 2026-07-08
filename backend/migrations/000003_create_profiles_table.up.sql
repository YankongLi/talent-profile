CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    headline TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    target_roles_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    visibility TEXT NOT NULL DEFAULT 'draft',
    template_id TEXT NOT NULL DEFAULT 'default',
    theme_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT profiles_visibility_valid CHECK (visibility IN ('draft', 'unlisted', 'public')),
    CONSTRAINT profiles_target_roles_array CHECK (jsonb_typeof(target_roles_json) = 'array'),
    CONSTRAINT profiles_theme_object CHECK (jsonb_typeof(theme_json) = 'object'),
    CONSTRAINT profiles_template_id_not_blank CHECK (btrim(template_id) <> '')
);

CREATE INDEX profiles_visibility_idx
    ON profiles (visibility, published_at DESC);

CREATE TRIGGER profiles_set_updated_at
    BEFORE UPDATE ON profiles
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
