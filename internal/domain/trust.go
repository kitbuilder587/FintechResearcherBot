package domain

type TrustLevel string

const (
	TrustHigh   TrustLevel = "high"
	TrustMedium TrustLevel = "medium"
	TrustLow    TrustLevel = "low"
)

func (t TrustLevel) IsValid() bool {
	switch t {
	case TrustHigh, TrustMedium, TrustLow:
		return true
	default:
		return false
	}
}

func (t TrustLevel) String() string {
	return string(t)
}
