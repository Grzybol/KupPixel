package bcrypt

import "testing"

func TestGenerateFromPasswordRoundTrip(t *testing.T) {
	password := []byte("correct horse battery staple")
	hash, err := GenerateFromPassword(password, DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword error: %v", err)
	}

	if err := CompareHashAndPassword(hash, password); err != nil {
		t.Fatalf("CompareHashAndPassword mismatch: %v", err)
	}
}

func TestCompareHashAndPasswordRejectsWrongPassword(t *testing.T) {
	password := []byte("p@ssw0rd")
	hash, err := GenerateFromPassword(password, DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword error: %v", err)
	}

	wrong := []byte("not-the-password")
	if err := CompareHashAndPassword(hash, wrong); err != ErrMismatchedHashAndPassword {
		if err == nil {
			t.Fatal("CompareHashAndPassword accepted wrong password")
		}
		t.Fatalf("CompareHashAndPassword returned unexpected error: %v", err)
	}
}
