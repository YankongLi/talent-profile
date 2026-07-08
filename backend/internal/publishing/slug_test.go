package publishing

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name string
		slug string
		want error
	}{
		{name: "valid letters", slug: "zhangsan", want: nil},
		{name: "valid with digits", slug: "zhangsan123", want: nil},
		{name: "valid with hyphen", slug: "zhang-san", want: nil},
		{name: "empty", slug: "", want: ErrSlugRequired},
		{name: "too short", slug: "ab", want: ErrSlugTooShort},
		{name: "too long", slug: strings.Repeat("a", MaxSlugLength+1), want: ErrSlugTooLong},
		{name: "starts with hyphen", slug: "-zhangsan", want: ErrSlugInvalidHyphen},
		{name: "ends with hyphen", slug: "zhangsan-", want: ErrSlugInvalidHyphen},
		{name: "uppercase", slug: "Zhangsan", want: ErrSlugInvalidChar},
		{name: "underscore", slug: "zhang_san", want: ErrSlugInvalidChar},
		{name: "unicode", slug: "张三", want: ErrSlugInvalidChar},
		{name: "unicode long enough", slug: "张三abc", want: ErrSlugInvalidChar},
		{name: "reserved admin", slug: "admin", want: ErrSlugReserved},
		{name: "reserved api", slug: "api", want: ErrSlugReserved},
		{name: "reserved login", slug: "login", want: ErrSlugReserved},
		{name: "reserved static", slug: "static", want: ErrSlugReserved},
		{name: "reserved www", slug: "www", want: ErrSlugReserved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSlug(tt.slug)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSlug(%q) error = %v, want %v", tt.slug, err, tt.want)
			}
		})
	}
}

func TestIsReservedSlug(t *testing.T) {
	if !IsReservedSlug("api") {
		t.Fatal("api should be reserved")
	}
	if IsReservedSlug("zhangsan") {
		t.Fatal("zhangsan should not be reserved")
	}
}
