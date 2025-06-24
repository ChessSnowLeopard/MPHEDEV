package utils

type ParticipantInfo struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type PublicKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}

type SecretKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}

type GaloisKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	GalEl         uint64 `json:"gal_el"`
	ShareData     string `json:"share_data"`
}
