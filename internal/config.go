package internal

// Configuration struct for app settings
type Config struct {
	TelegramToken  string   `json:"telegram_token"`
	ChatID         string   `json:"chat_id"`
	AuthLogPath    string   `json:"auth_log_path"`
	CameraDevice   int      `json:"camera_device"`
	CheckInterval  int      `json:"check_interval"`
	SaveDir        string   `json:"save_dir"`
	StealthMode    bool     `json:"stealth_mode"`
	PatternStrings []string `json:"pattern_strings"`
}
