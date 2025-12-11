package crypto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// EncryptorTestSuite is the test suite for Encryptor
type EncryptorTestSuite struct {
	suite.Suite
	encryptor *Encryptor
	validKey  string
}

func (suite *EncryptorTestSuite) SetupTest() {
	suite.validKey = "12345678901234567890123456789012" // 32 bytes
	var err error
	suite.encryptor, err = NewEncryptor(suite.validKey)
	suite.Require().NoError(err)
}

// TestNewEncryptor tests the Encryptor constructor
func (suite *EncryptorTestSuite) TestNewEncryptor_ValidKey() {
	key := "12345678901234567890123456789012" // 32 bytes
	enc, err := NewEncryptor(key)
	suite.NoError(err)
	suite.NotNil(enc)
}

func (suite *EncryptorTestSuite) TestNewEncryptor_InvalidKeyTooShort() {
	key := "shortkey"
	enc, err := NewEncryptor(key)
	suite.Error(err)
	suite.Nil(enc)
	suite.Contains(err.Error(), "32 bytes")
}

func (suite *EncryptorTestSuite) TestNewEncryptor_InvalidKeyTooLong() {
	key := "1234567890123456789012345678901234567890" // 40 bytes
	enc, err := NewEncryptor(key)
	suite.Error(err)
	suite.Nil(enc)
}

func (suite *EncryptorTestSuite) TestNewEncryptor_EmptyKey() {
	enc, err := NewEncryptor("")
	suite.Error(err)
	suite.Nil(enc)
}

// TestEncryptDecrypt tests the Encrypt and Decrypt methods
func (suite *EncryptorTestSuite) TestEncryptDecrypt_EmptyString() {
	plaintext := ""
	ciphertext, err := suite.encryptor.Encrypt(plaintext)
	suite.NoError(err)
	suite.NotEmpty(ciphertext)

	decrypted, err := suite.encryptor.Decrypt(ciphertext)
	suite.NoError(err)
	suite.Equal(plaintext, decrypted)
}

func (suite *EncryptorTestSuite) TestEncryptDecrypt_ShortString() {
	plaintext := "hello"
	ciphertext, err := suite.encryptor.Encrypt(plaintext)
	suite.NoError(err)
	suite.NotEmpty(ciphertext)

	decrypted, err := suite.encryptor.Decrypt(ciphertext)
	suite.NoError(err)
	suite.Equal(plaintext, decrypted)
}

func (suite *EncryptorTestSuite) TestEncryptDecrypt_LongString() {
	plaintext := strings.Repeat("a", 2000) // 2KB string
	ciphertext, err := suite.encryptor.Encrypt(plaintext)
	suite.NoError(err)
	suite.NotEmpty(ciphertext)

	decrypted, err := suite.encryptor.Decrypt(ciphertext)
	suite.NoError(err)
	suite.Equal(plaintext, decrypted)
}

func (suite *EncryptorTestSuite) TestEncryptDecrypt_SpecialCharacters() {
	plaintext := "hello世界🔐!@#$%^&*()"
	ciphertext, err := suite.encryptor.Encrypt(plaintext)
	suite.NoError(err)

	decrypted, err := suite.encryptor.Decrypt(ciphertext)
	suite.NoError(err)
	suite.Equal(plaintext, decrypted)
}

func (suite *EncryptorTestSuite) TestEncrypt_UniqueNonce() {
	plaintext := "same plaintext"

	// Encrypt same text multiple times
	ciphertexts := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ct, err := suite.encryptor.Encrypt(plaintext)
		suite.NoError(err)
		ciphertexts[ct] = true
	}

	// All ciphertexts should be unique due to random nonce
	suite.Equal(100, len(ciphertexts), "All ciphertexts should be unique")
}

func (suite *EncryptorTestSuite) TestDecrypt_InvalidBase64() {
	_, err := suite.encryptor.Decrypt("not-valid-base64!!!")
	suite.Error(err)
	suite.Contains(err.Error(), "decode")
}

func (suite *EncryptorTestSuite) TestDecrypt_CorruptedCiphertext() {
	plaintext := "test data"
	ciphertext, err := suite.encryptor.Encrypt(plaintext)
	suite.NoError(err)

	// Corrupt the ciphertext by modifying a byte
	corrupted := []byte(ciphertext)
	if len(corrupted) > 10 {
		corrupted[10] = corrupted[10] ^ 0xFF
	}

	_, err = suite.encryptor.Decrypt(string(corrupted))
	suite.Error(err)
}

func (suite *EncryptorTestSuite) TestDecrypt_TruncatedCiphertext() {
	plaintext := "test data"
	ciphertext, err := suite.encryptor.Encrypt(plaintext)
	suite.NoError(err)

	// Truncate the ciphertext
	truncated := ciphertext[:len(ciphertext)/2]

	_, err = suite.encryptor.Decrypt(truncated)
	suite.Error(err)
}

func (suite *EncryptorTestSuite) TestDecrypt_CiphertextTooShort() {
	_, err := suite.encryptor.Decrypt("YWJjZA==") // "abcd" base64 encoded
	suite.Error(err)
	suite.Contains(err.Error(), "too short")
}

func (suite *EncryptorTestSuite) TestDecrypt_WrongKey() {
	plaintext := "secret message"
	ciphertext, err := suite.encryptor.Encrypt(plaintext)
	suite.NoError(err)

	// Create new encryptor with different key
	differentKey := "abcdefghijklmnopqrstuvwxyz123456"
	differentEncryptor, err := NewEncryptor(differentKey)
	suite.NoError(err)

	_, err = differentEncryptor.Decrypt(ciphertext)
	suite.Error(err)
}

// Run the test suite
func TestEncryptorTestSuite(t *testing.T) {
	suite.Run(t, new(EncryptorTestSuite))
}

// Table-driven tests for HashPassword and CheckPassword
func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"empty password", ""},
		{"short password", "pass"},
		{"normal password", "password123"},
		{"long password", strings.Repeat("a", 72)}, // bcrypt max is 72 bytes
		{"special characters", "p@ssw0rd!#$%"},
		{"unicode password", "пароль123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			require.NoError(t, err)
			assert.NotEmpty(t, hash)
			assert.NotEqual(t, tt.password, hash)

			// Verify the hash starts with bcrypt prefix
			assert.True(t, strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$"))
		})
	}
}

func TestHashPassword_DifferentHashesForSamePassword(t *testing.T) {
	password := "testpassword"
	hashes := make(map[string]bool)

	for i := 0; i < 10; i++ {
		hash, err := HashPassword(password)
		require.NoError(t, err)
		hashes[hash] = true
	}

	// All hashes should be unique due to random salt
	assert.Equal(t, 10, len(hashes), "All hashes should be unique")
}

func TestCheckPassword(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		checkPassword  string
		expectedResult bool
	}{
		{"correct password", "password123", "password123", true},
		{"wrong password", "password123", "wrongpassword", false},
		{"empty vs non-empty", "", "password", false},
		{"case sensitive", "Password", "password", false},
		{"whitespace matters", "pass word", "password", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			require.NoError(t, err)

			result := CheckPassword(tt.checkPassword, hash)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestCheckPassword_InvalidHash(t *testing.T) {
	result := CheckPassword("password", "invalid-hash")
	assert.False(t, result)
}

// Tests for EncryptWithSalt/DecryptWithSalt
func TestEncryptDecryptWithSalt(t *testing.T) {
	key := []byte("12345678901234567890123456789012")
	salt := "unique_salt_value"

	tests := []struct {
		name      string
		plaintext string
		salt      string
	}{
		{"normal text", "secret data", salt},
		{"empty text", "", salt},
		{"long text", strings.Repeat("x", 1000), salt},
		{"unicode text", "тестовые данные", salt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := EncryptWithSalt(tt.plaintext, tt.salt, key)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)

			decrypted, err := DecryptWithSalt(ciphertext, tt.salt, key)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestDecryptWithSalt_WrongSalt(t *testing.T) {
	key := []byte("12345678901234567890123456789012")
	plaintext := "secret message"
	correctSalt := "correct_salt"
	wrongSalt := "wrong_salt"

	ciphertext, err := EncryptWithSalt(plaintext, correctSalt, key)
	require.NoError(t, err)

	decrypted, err := DecryptWithSalt(ciphertext, wrongSalt, key)
	require.NoError(t, err)
	// The decrypted text will be different because salt is part of the plaintext
	assert.NotEqual(t, plaintext, decrypted)
}

func TestEncryptWithSalt_InvalidKey(t *testing.T) {
	shortKey := []byte("short")
	_, err := EncryptWithSalt("test", "salt", shortKey)
	assert.Error(t, err)
}

func TestDecryptWithSalt_InvalidCiphertext(t *testing.T) {
	key := []byte("12345678901234567890123456789012")

	_, err := DecryptWithSalt("invalid-base64!!!", "salt", key)
	assert.Error(t, err)
}

// Tests for GenerateRandomKey
func TestGenerateRandomKey(t *testing.T) {
	tests := []struct {
		name     string
		length   int
		expected int // hex encoded length = 2 * byte length
	}{
		{"16 bytes", 16, 32},
		{"32 bytes", 32, 64},
		{"64 bytes", 64, 128},
		{"1 byte", 1, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateRandomKey(tt.length)
			require.NoError(t, err)
			assert.Len(t, key, tt.expected)
		})
	}
}

func TestGenerateRandomKey_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)

	for i := 0; i < 100; i++ {
		key, err := GenerateRandomKey(32)
		require.NoError(t, err)
		keys[key] = true
	}

	assert.Equal(t, 100, len(keys), "All keys should be unique")
}

// Tests for GenerateRandomBytes
func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"16 bytes", 16},
		{"32 bytes", 32},
		{"64 bytes", 64},
		{"1 byte", 1},
		{"256 bytes", 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := GenerateRandomBytes(tt.length)
			require.NoError(t, err)
			assert.Len(t, bytes, tt.length)
		})
	}
}

func TestGenerateRandomBytes_Uniqueness(t *testing.T) {
	bytesSet := make(map[string]bool)

	for i := 0; i < 100; i++ {
		b, err := GenerateRandomBytes(32)
		require.NoError(t, err)
		bytesSet[string(b)] = true
	}

	assert.Equal(t, 100, len(bytesSet), "All byte arrays should be unique")
}

// Tests for SHA256Hash
func TestSHA256Hash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // pre-computed SHA256 hashes
	}{
		{
			"empty string",
			"",
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			"hello",
			"hello",
			"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			"test",
			"test",
			"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SHA256Hash(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSHA256Hash_Deterministic(t *testing.T) {
	input := "test data"
	hash1 := SHA256Hash(input)
	hash2 := SHA256Hash(input)

	assert.Equal(t, hash1, hash2, "SHA256 should be deterministic")
}

func TestSHA256Hash_DifferentInputs(t *testing.T) {
	inputs := []string{"input1", "input2", "input3", "Input1", "INPUT1"}
	hashes := make(map[string]bool)

	for _, input := range inputs {
		hash := SHA256Hash(input)
		hashes[hash] = true
	}

	assert.Equal(t, len(inputs), len(hashes), "Different inputs should produce different hashes")
}

func TestSHA256Hash_Length(t *testing.T) {
	hash := SHA256Hash("any input")
	assert.Len(t, hash, 64, "SHA256 hex output should be 64 characters")
}

// Tests for TokenGenerator
func TestTokenGenerator(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"16 bytes token", 16},
		{"32 bytes token", 32},
		{"64 bytes token", 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewTokenGenerator(tt.length)
			token, err := gen.Generate()
			require.NoError(t, err)
			assert.NotEmpty(t, token)
		})
	}
}

func TestGenerateSecureToken(t *testing.T) {
	token, err := GenerateSecureToken(32)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestGenerateSecureToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token, err := GenerateSecureToken(32)
		require.NoError(t, err)
		tokens[token] = true
	}

	assert.Equal(t, 100, len(tokens), "All tokens should be unique")
}

// Benchmark tests
func BenchmarkEncrypt(b *testing.B) {
	key := "12345678901234567890123456789012"
	enc, _ := NewEncryptor(key)
	plaintext := "This is a test message for benchmarking encryption performance"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Encrypt(plaintext)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	key := "12345678901234567890123456789012"
	enc, _ := NewEncryptor(key)
	plaintext := "This is a test message for benchmarking decryption performance"
	ciphertext, _ := enc.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Decrypt(ciphertext)
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "testpassword123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password)
	}
}

func BenchmarkCheckPassword(b *testing.B) {
	password := "testpassword123"
	hash, _ := HashPassword(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CheckPassword(password, hash)
	}
}

func BenchmarkSHA256Hash(b *testing.B) {
	data := strings.Repeat("test data ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SHA256Hash(data)
	}
}

func BenchmarkGenerateRandomKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateRandomKey(32)
	}
}

func BenchmarkEncryptWithSalt(b *testing.B) {
	key := []byte("12345678901234567890123456789012")
	plaintext := "This is a test message for benchmarking"
	salt := "unique_salt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncryptWithSalt(plaintext, salt, key)
	}
}

