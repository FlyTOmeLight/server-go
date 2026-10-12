package datatypes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupWithExistingKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"Key": "Value"}
	value, ok := m.Lookup("Key")
	assert.True(t, ok)
	assert.Equal(t, "Value", value)
}

func TestLookupWithNonExistingKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"Key": "Value"}
	_, ok := m.Lookup("NonExistingKey")
	assert.False(t, ok)
}

func TestLookupWithEmptyMap(t *testing.T) {
	m := CaseInsensitiveMap[string]{}
	_, ok := m.Lookup("Key")
	assert.False(t, ok)
}

func TestLookupWithCaseInsensitiveKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"kubeconfig": "Value"}
	value, ok := m.Lookup("kubeConfig")
	assert.True(t, ok)
	assert.Equal(t, "Value", value)
}

func TestLookupWithCaseInsensitiveKeyFromStruct(t *testing.T) {
	type TestConfig struct {
		Config CaseInsensitiveMap[string] `json:"config"`
	}
	config := TestConfig{
		Config: CaseInsensitiveMap[string]{"kubeconfig": "Value"},
	}

	value, ok := config.Config.Lookup("kubeConfig")
	assert.True(t, ok)
	assert.Equal(t, "Value", value)

	configCamel := TestConfig{
		Config: CaseInsensitiveMap[string]{"kubeConfig": "Value"},
	}
	value, ok = configCamel.Config.Lookup("kubeConfig")
	assert.True(t, ok)
	assert.Equal(t, "Value", value)
}

func TestLookupWithCanonicalHeaderKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"Content-Type": "application/json"}
	value, ok := m.Lookup("content-type")
	assert.True(t, ok)
	assert.Equal(t, "application/json", value)
}

func TestDeleteWithExistingKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"Key": "Value"}
	m.Delete("Key")
	_, ok := m.Lookup("Key")
	assert.False(t, ok)
}

func TestDeleteWithNonExistingKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"Key": "Value"}
	m.Delete("NonExistingKey")
	_, ok := m.Lookup("Key")
	assert.True(t, ok)
}

func TestDeleteWithEmptyMap(t *testing.T) {
	m := CaseInsensitiveMap[string]{}
	m.Delete("Key")
	_, ok := m.Lookup("Key")
	assert.False(t, ok)
}

func TestDeleteWithCaseInsensitiveKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"Key": "Value"}
	m.Delete("key")
	_, ok := m.Lookup("Key")
	assert.False(t, ok)
}

func TestDeleteWithCanonicalHeaderKey(t *testing.T) {
	m := CaseInsensitiveMap[string]{"Content-Type": "application/json"}
	m.Delete("content-type")
	_, ok := m.Lookup("Content-Type")
	assert.False(t, ok)
}
