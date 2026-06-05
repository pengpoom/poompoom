package businessapikeys

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusRevoked  = "revoked"
)

type APIKey struct {
	ID                 string   `json:"id"`
	UserID             string   `json:"userId"`
	Name               string   `json:"name"`
	KeyPrefix          string   `json:"keyPrefix"`
	KeyLast4           string   `json:"keyLast4"`
	Status             string   `json:"status"`
	CreditLimit        int64    `json:"creditLimit"`
	UsedCredits        int64    `json:"usedCredits"`
	RateLimitPerMinute int      `json:"rateLimitPerMinute"`
	ConcurrencyLimit   int      `json:"concurrencyLimit"`
	AllowedModels      []string `json:"allowedModels"`
	LastUsedAt         string   `json:"lastUsedAt,omitempty"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	RevokedAt          string   `json:"revokedAt,omitempty"`
	KeyCipher          string   `json:"-"`
	Plaintext          string   `json:"plaintext,omitempty"`
}
