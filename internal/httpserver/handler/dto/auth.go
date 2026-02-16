package dto

// RefreshRequest — тело запроса для POST /api/user/refresh и POST /api/user/logout.
// Используется один и тот же контракт: клиент передает refresh token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}
