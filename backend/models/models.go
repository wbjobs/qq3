package models

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB
var RDB *redis.Client
var Ctx = context.Background()

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Email     string    `gorm:"size:100" json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Device struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	DeviceID  string    `gorm:"uniqueIndex;size:100;not null" json:"device_id"`
	DeviceName string   `gorm:"size:100;not null" json:"device_name"`
	DeviceType string   `gorm:"size:20;not null" json:"device_type"`
	LastIP    string    `gorm:"size:50" json:"last_ip"`
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
}

type ClipboardItem struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	SeqID        int64     `gorm:"index;not null" json:"seq_id"`
	Content      string    `gorm:"type:text;not null" json:"content"`
	Translation  string    `gorm:"type:text" json:"translation,omitempty"`
	SourceDevice string    `gorm:"size:100" json:"source_device"`
	DeviceName   string    `gorm:"size:100" json:"device_name"`
	ContentType  string    `gorm:"size:20;default:'text'" json:"content_type"`
	IsTranslated bool      `gorm:"default:false" json:"is_translated"`
	IsFiltered   bool      `gorm:"default:false" json:"is_filtered"`
	FilteredHits string    `gorm:"type:text" json:"filtered_hits,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserSettings struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	SilentMode   bool      `gorm:"default:false" json:"silent_mode"`
	FilterEnable bool      `gorm:"default:true" json:"filter_enable"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func GetNextSeqID(userID uint) (int64, error) {
	key := fmt.Sprintf("seq:user:%d", userID)
	return RDB.Incr(Ctx, key).Result()
}

func (u *User) HashPassword(password string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(bytes)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

func InitDB(dsn string) {
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect MySQL:", err)
	}

	err = DB.AutoMigrate(&User{}, &Device{}, &ClipboardItem{}, &UserSettings{})
	if err != nil {
		log.Fatal("Failed to migrate:", err)
	}

	log.Println("MySQL initialized successfully")
}

func InitRedis(addr, password string, db int) {
	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	_, err := RDB.Ping(Ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect Redis:", err)
	}
	log.Println("Redis initialized successfully")
}

func PushClipboardToRedis(userID uint, item *ClipboardItem) error {
	key := fmt.Sprintf("clipboard:user:%d:recent", userID)

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	pipe := RDB.TxPipeline()
	pipe.LPush(Ctx, key, data)
	pipe.LTrim(Ctx, key, 0, 9)
	pipe.Expire(Ctx, key, 7*24*time.Hour)
	_, err = pipe.Exec(Ctx)
	return err
}

func GetRecentClipboardFromRedis(userID uint) ([]ClipboardItem, error) {
	key := fmt.Sprintf("clipboard:user:%d:recent", userID)
	results, err := RDB.LRange(Ctx, key, 0, 9).Result()
	if err != nil {
		return nil, err
	}

	items := make([]ClipboardItem, 0, len(results))
	for _, r := range results {
		var item ClipboardItem
		if err := json.Unmarshal([]byte(r), &item); err == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func CreateClipboardItem(item *ClipboardItem) error {
	if err := DB.Create(item).Error; err != nil {
		return err
	}
	return PushClipboardToRedis(item.UserID, item)
}

func GetClipboardHistory(userID uint, limit, offset int) ([]ClipboardItem, int64, error) {
	var items []ClipboardItem
	var total int64

	DB.Model(&ClipboardItem{}).Where("user_id = ?", userID).Count(&total)
	err := DB.Where("user_id = ?", userID).Order("seq_id DESC").
		Limit(limit).Offset(offset).Find(&items).Error

	if len(items) == 0 {
		redisItems, _ := GetRecentClipboardFromRedis(userID)
		if len(redisItems) > 0 {
			return redisItems, int64(len(redisItems)), nil
		}
	}

	return items, total, err
}

func GetUserSettings(userID uint) (*UserSettings, error) {
	var settings UserSettings
	err := DB.Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			settings = UserSettings{
				UserID:       userID,
				SilentMode:   false,
				FilterEnable: true,
			}
			DB.Create(&settings)
			return &settings, nil
		}
		return nil, err
	}
	return &settings, nil
}

func UpdateUserSettings(settings *UserSettings) error {
	settings.UpdatedAt = time.Now()
	return DB.Save(settings).Error
}

func SetSilentMode(userID uint, silent bool) (*UserSettings, error) {
	settings, err := GetUserSettings(userID)
	if err != nil {
		return nil, err
	}
	settings.SilentMode = silent
	settings.UpdatedAt = time.Now()
	if err := DB.Save(settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func SetFilterEnable(userID uint, enable bool) (*UserSettings, error) {
	settings, err := GetUserSettings(userID)
	if err != nil {
		return nil, err
	}
	settings.FilterEnable = enable
	settings.UpdatedAt = time.Now()
	if err := DB.Save(settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}
