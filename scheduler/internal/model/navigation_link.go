package model

import "time"

// NavigationLink 导航快捷链接配置（Dashboard 首页展示）
type NavigationLink struct {
	ID            int64     `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	Description   string    `db:"description" json:"description"`
	URL           string    `db:"url" json:"url"`
	Width         string    `db:"width" json:"width"`                     // 'third' | 'half' | 'full'
	DisplayMode   string    `db:"display_mode" json:"display_mode"`       // 'compact' | 'normal' | 'banner'
	HeightPx      int       `db:"height_px" json:"height_px"`             // 0 表示使用 display_mode 基准值
	IconData      string    `db:"icon_data" json:"icon_data"`             // base64 data: URL，可空
	IconFetchedAt time.Time `db:"icon_fetched_at" json:"icon_fetched_at"` // 最近一次抓取时间，零值表示未抓过
	SortOrder     int       `db:"sort_order" json:"sort_order"`           // 升序排列，越小越靠前
	Enabled       int       `db:"enabled" json:"enabled"`                 // 0=禁用 1=启用
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

// NavigationReorderItem 批量排序更新项
type NavigationReorderItem struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}

// NavigationLinkWidthThird 三分之一列宽
const NavigationLinkWidthThird = "third"

// NavigationLinkWidthHalf 二分之一列宽
const NavigationLinkWidthHalf = "half"

// NavigationLinkWidthFull 整行宽度
const NavigationLinkWidthFull = "full"

// NavigationLinkDisplayModeCompact 紧凑密度
const NavigationLinkDisplayModeCompact = "compact"

// NavigationLinkDisplayModeNormal 标准密度
const NavigationLinkDisplayModeNormal = "normal"

// NavigationLinkDisplayModeBanner 大卡片密度
const NavigationLinkDisplayModeBanner = "banner"

// DisplayModeBaseHeight 各 display_mode 的基准高度（单位 px）
var DisplayModeBaseHeight = map[string]int{
	NavigationLinkDisplayModeCompact: 52,
	NavigationLinkDisplayModeNormal:  68,
	NavigationLinkDisplayModeBanner:  100,
}

// ValidWidths 合法的 width 值集合
var ValidWidths = map[string]bool{
	NavigationLinkWidthThird: true,
	NavigationLinkWidthHalf:  true,
	NavigationLinkWidthFull:  true,
}

// ValidDisplayModes 合法的 display_mode 值集合
var ValidDisplayModes = map[string]bool{
	NavigationLinkDisplayModeCompact: true,
	NavigationLinkDisplayModeNormal:  true,
	NavigationLinkDisplayModeBanner:  true,
}
