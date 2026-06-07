package main

import (
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Global singletons
var (
	DB    *gorm.DB
	RDB   *redis.Client
	WAuth *webauthn.WebAuthn
)
