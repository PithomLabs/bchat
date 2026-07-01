package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePolicySettingsRequireContactOnFallback(t *testing.T) {
	content := `
<!-- @settings: require_contact_on_fallback: false -->
## Settings
`

	policy, err := NewParser().ParsePolicy(content, 42, "external")
	require.NoError(t, err)
	require.NotNil(t, policy.Audience)
	require.False(t, policy.Audience.RequireContactOnFallback)
	require.Equal(t, 4, policy.Audience.EmergencyUrgencyThreshold)
	require.Equal(t, 0.85, policy.Audience.EscalationConfidenceThreshold)
}

func TestParsePolicySettingsAndThresholdsShareAudience(t *testing.T) {
	content := `
<!-- @settings: require_contact_on_fallback: false -->
## Settings

<!-- @thresholds -->
## Thresholds

| Metric | Threshold | Action |
| Emergency Urgency | >= 3 | Route to emergency flow |
| Escalation Confidence | >= 0.75 | Confirm escalation |
`

	policy, err := NewParser().ParsePolicy(content, 42, "external")
	require.NoError(t, err)
	require.NotNil(t, policy.Audience)
	require.False(t, policy.Audience.RequireContactOnFallback)
	require.Equal(t, 3, policy.Audience.EmergencyUrgencyThreshold)
	require.InEpsilon(t, 0.75, policy.Audience.EscalationConfidenceThreshold, 0.0001)
}

func TestParsePolicyIdentityAndSettingsShareAudience(t *testing.T) {
	content := `
<!-- @settings: require_contact_on_fallback: false -->
## Settings

<!-- @identity -->
## Identity

**Role:** Museum Guide
**Tone:** Warm
`

	policy, err := NewParser().ParsePolicy(content, 42, "external")
	require.NoError(t, err)
	require.NotNil(t, policy.Audience)
	require.False(t, policy.Audience.RequireContactOnFallback)
	require.Equal(t, "Museum Guide", policy.Audience.Role)
	require.Equal(t, "Warm", policy.Audience.Tone)
}
