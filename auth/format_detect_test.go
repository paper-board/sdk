package auth

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		name  string
		token string
		want  AuthMode
	}{
		{
			name:  "valid live api key",
			token: "pbk_live_ABCDEFGHJKMNPQRSTVWXYZ23456789AB",
			want:  APIKey,
		},
		{
			name:  "valid test api key",
			token: "pbk_test_ABCDEFGHJKMNPQRSTVWXYZ23456789AB",
			want:  APIKey,
		},
		{
			name:  "jwt bearer",
			token: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.sig",
			want:  JWT,
		},
		{
			name:  "wrong prefix",
			token: "pbl_live_ABCDEFGHJKMNPQRSTVWXYZ23456789AB",
			want:  JWT,
		},
		{
			name:  "too short suffix (31 chars)",
			token: "pbk_live_ABCDEFGHJKMNPQRSTVWXYZ2345678A",
			want:  JWT,
		},
		{
			name:  "too long suffix (33 chars)",
			token: "pbk_live_ABCDEFGHJKMNPQRSTVWXYZ23456789ABC",
			want:  JWT,
		},
		{
			name:  "invalid crockford chars (lowercase)",
			token: "pbk_live_abcdefghjkmnpqrstvwxyz23456789ab",
			want:  JWT,
		},
		{
			name:  "invalid env segment",
			token: "pbk_prod_ABCDEFGHJKMNPQRSTVWXYZ23456789AB",
			want:  JWT,
		},
		{
			name:  "empty string",
			token: "",
			want:  JWT,
		},
		{
			name:  "exactly 32 uppercase crockford live",
			token: "pbk_live_23456789ABCDEFGHJKMNPQRSTVWXYZ23",
			want:  APIKey,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detect(tc.token)
			if got != tc.want {
				t.Errorf("detect(%q) = %v; want %v", tc.token, got, tc.want)
			}
		})
	}
}
