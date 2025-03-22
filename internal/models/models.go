package models

// SessionInfo chứa thông tin session và đăng nhập
type SessionInfo struct {
	PHPSESSID              string `json:"PHPSESSID"`
	RequestVerificationToken string `json:"RequestVerificationToken"`
	UniqueSessionId        string `json:"UniqueSessionId"`
	FingerIDX              string `json:"FingerIDX"`
	IdyKey                 string `json:"IdyKey"`
	Timestamp              int64  `json:"Timestamp"`
}

// SliderCaptchaRequest chứa dữ liệu để kiểm tra captcha
type SliderCaptchaRequest struct {
	Trail []int `json:"Trail"`
}

// SliderCaptchaResponse chứa response từ việc kiểm tra captcha
type SliderCaptchaResponse struct {
	Success bool       `json:"Success"`
	Data    CaptchaInfo `json:"Data"`
}

// LoginRequest chứa dữ liệu cho request đăng nhập
type LoginRequest struct {
	AccountID           string `json:"AccountID"`
	AccountPWD          string `json:"AccountPWD"`
	ProtectCode         string `json:"ProtectCode"`
	LocalStorgeCookie   string `json:"LocalStorgeCookie"`
	FingerIDX           string `json:"FingerIDX"`
	ScreenResolution    string `json:"ScreenResolution"`
	ShowSliderCaptcha   bool   `json:"ShowSliderCaptcha"`
	ShowPhoneVerify     bool   `json:"ShowPhoneVerify"`
	VerifySliderCaptcha bool   `json:"VerifySliderCaptcha"`
	UniqueSessionId     string `json:"UniqueSessionId"`
	LoginVerification   int    `json:"LoginVerification"`
	IdyKey              string `json:"IdyKey"`
}

// LoginResponse chứa kết quả từ request đăng nhập
type LoginResponse struct {
	Success bool        `json:"Success"`
	Error   interface{} `json:"Error"`
	Data    interface{} `json:"Data"`
}

// Error chứa thông tin lỗi từ API
type Error struct {
	Code    interface{} `json:"Code"`
	Message string      `json:"Message"`
}

// CaptchaInfo chứa thông tin captcha slider
type CaptchaInfo struct {
	Slider     string `json:"Slider"`
	Background string `json:"Background"`
}

// SliderCaptchaVerifyResponse phản hồi khi xác minh captcha
type SliderCaptchaVerifyResponse struct {
	Success bool   `json:"Success"`
	Data    struct {
		Message string `json:"Message"`
	} `json:"Data"`
} 