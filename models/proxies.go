package models

import (
	"gorm.io/gorm"
	"time"
)

type Proxies struct {
	ID        int64   `gorm:"column:id" db:"id" json:"id" form:"id"`
	Name      string  `gorm:"column:name" db:"name" json:"name" form:"name"`
	Bandwidth float64 `gorm:"column:bandwidth" db:"bandwidth" json:"bandwidth" form:"bandwidth"`
	TTFB      float64 `gorm:"column:ttfb" db:"ttfb" json:"ttfb" form:"ttfb"`
	Info      string  `gorm:"column:info" db:"info" json:"info" form:"info"`
	Status    int64   `gorm:"column:status" db:"status" json:"status" form:"status"`
	GPT       int     `gorm:"column:gpt" db:"gpt" json:"gpt" form:"gpt"`
	Server    string  `gorm:"column:server" db:"server" json:"server" form:"server"`
	MD5       string  `gorm:"column:md5" db:"md5" json:"md5" form:"md5"`
	Region    string  `gorm:"column:region" db:"region" json:"region" form:"region"`
	Port      int     `gorm:"column:port" db:"port" json:"port" form:"port"`
	Fault     int     `gorm:"column:fault" db:"fault" json:"fault" form:"fault"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type ProxiesGroup struct {
	H []*Proxies
	C []*Proxies
	L []*Proxies
}
