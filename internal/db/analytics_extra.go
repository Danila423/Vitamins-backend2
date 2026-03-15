package db

import "context"

const getUserAnalyticsConsent = `
SELECT analytics_consent
FROM users
WHERE id = $1
`

func (q *Queries) GetUserAnalyticsConsent(ctx context.Context, userID int64) (bool, error) {
	var consent bool
	err := q.db.QueryRow(ctx, getUserAnalyticsConsent, userID).Scan(&consent)
	return consent, err
}

const updateUserAnalyticsConsent = `
UPDATE users
SET analytics_consent = $1,
    analytics_consent_at = CASE WHEN $1 THEN now() ELSE NULL END
WHERE id = $2
`

func (q *Queries) UpdateUserAnalyticsConsent(ctx context.Context, userID int64, consent bool) error {
	_, err := q.db.Exec(ctx, updateUserAnalyticsConsent, consent, userID)
	return err
}
