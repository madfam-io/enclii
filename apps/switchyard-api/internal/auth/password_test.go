package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "password123",
			wantErr:  false,
		},
		{
			name:     "long password",
			password: strings.Repeat("a", 72),
			wantErr:  false,
		},
		{
			name:     "password with special characters",
			password: "p@ssw0rd!#$%",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false, // bcrypt handles empty passwords
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)

			if tt.wantErr {
				if err == nil {
					t.Error("HashPassword() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("HashPassword() unexpected error: %v", err)
				return
			}

			if hash == "" {
				t.Error("HashPassword() returned empty hash")
				return
			}

			if hash == tt.password {
				t.Error("HashPassword() returned password in plain text")
			}

			// Hash should start with bcrypt prefix
			if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") && !strings.HasPrefix(hash, "$2y$") {
				t.Errorf("HashPassword() hash doesn't have bcrypt prefix: %s", hash)
			}
		})
	}
}

func TestHashPassword_Uniqueness(t *testing.T) {
	password := "test123"

	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)

	if err1 != nil || err2 != nil {
		t.Fatalf("HashPassword() errors: %v, %v", err1, err2)
	}

	// Hashes should be different due to random salt
	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes for same password (should use random salt)")
	}

	// But both should verify correctly
	if !CheckPasswordHash(password, hash1) {
		t.Error("CheckPasswordHash() failed to verify first hash")
	}
	if !CheckPasswordHash(password, hash2) {
		t.Error("CheckPasswordHash() failed to verify second hash")
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "password123"
	hash, _ := HashPassword(password)

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     hash,
			want:     true,
		},
		{
			name:     "incorrect password",
			password: "wrongpassword",
			hash:     hash,
			want:     false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			want:     false,
		},
		{
			name:     "invalid hash",
			password: password,
			hash:     "not-a-valid-hash",
			want:     false,
		},
		{
			name:     "empty hash",
			password: password,
			hash:     "",
			want:     false,
		},
		{
			name:     "case sensitive - different case",
			password: "Password123",
			hash:     hash,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPasswordHash(tt.password, tt.hash)
			if got != tt.want {
				t.Errorf("CheckPasswordHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPasswordHash_EdgeCases(t *testing.T) {
	// Test with various password lengths
	lengths := []int{1, 10, 50, 72}

	for _, length := range lengths {
		password := strings.Repeat("a", length)
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() failed for length %d: %v", length, err)
		}

		if !CheckPasswordHash(password, hash) {
			t.Errorf("CheckPasswordHash() failed for password of length %d", length)
		}
	}
}

func TestCheckPasswordHash_SpecialCharacters(t *testing.T) {
	passwords := []string{
		"p@ssw0rd!",
		"пароль", // Cyrillic
		"密码",     // Chinese
		"🔒🔑",    // Emojis
		"pass\nword\twith\rwhitespace",
		"pass\"word'with`quotes",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("HashPassword() error: %v", err)
			}

			if !CheckPasswordHash(password, hash) {
				t.Errorf("CheckPasswordHash() failed for password: %s", password)
			}
		})
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "benchmark123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashPassword(password)
	}
}

func BenchmarkCheckPasswordHash(b *testing.B) {
	password := "benchmark123"
	hash, _ := HashPassword(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckPasswordHash(password, hash)
	}
}
