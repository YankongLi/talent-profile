CREATE TABLE profile_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    section_type TEXT NOT NULL,
    content_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_visible BOOLEAN NOT NULL DEFAULT true,
    is_user_confirmed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT profile_sections_type_not_blank CHECK (btrim(section_type) <> ''),
    CONSTRAINT profile_sections_content_object CHECK (jsonb_typeof(content_json) = 'object'),
    CONSTRAINT profile_sections_sort_order_non_negative CHECK (sort_order >= 0)
);

CREATE INDEX profile_sections_profile_order_idx
    ON profile_sections (profile_id, sort_order, created_at);

CREATE TRIGGER profile_sections_set_updated_at
    BEFORE UPDATE ON profile_sections
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
