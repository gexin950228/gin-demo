package models

import (
	conf "gin-demo/conf"
	"gin-demo/skywalking"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB
var ArticleDB *gorm.DB

func InitDB() {
	dsn := conf.DBConfig.User + ":" + conf.DBConfig.Password + "@tcp(" + conf.DBConfig.Host + ":" + conf.DBConfig.Port + ")/" + conf.DBConfig.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("数据库连接失败: " + err.Error())
	}

	// 文章库连接（与用户库在同一 MySQL 实例，仅 dbname 不同）
	// 注意：users 表在 gin_project 库，文章库的 articles/comments 等表通过 user_id 跨库关联，
	// 不能创建真实外键约束，因此禁用迁移时的外键创建（DisableForeignKeyConstraintWhenMigrating）
	articleDSN := conf.ArticleDBConfig.User + ":" + conf.ArticleDBConfig.Password + "@tcp(" + conf.ArticleDBConfig.Host + ":" + conf.ArticleDBConfig.Port + ")/" + conf.ArticleDBConfig.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
	ArticleDB, err = gorm.Open(mysql.Open(articleDSN), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		panic("文章库连接失败: " + err.Error())
	}

	// 自动迁移表结构：用户库只管 users，文章库管 articles/tags/article_edit_approvals/comments/notifications
	DB.AutoMigrate(&User{})
	ArticleDB.AutoMigrate(&Article{}, &Tag{}, &ArticleEditApproval{}, &Comment{}, &Notification{})

	// 回填存量评论的 root_id 和 reply_to_user_id（两级平铺展示所需字段）
	// 背景：这两个字段是后加的，AutoMigrate 只会新增列（默认 0），不会回填历史数据。
	// root_id=0 的老回复会被当成顶级评论显示，导致回复关系丢失，需一次性修复。
	backfillCommentRootID()

	// 清理 articles 库中历史遗留的跨库外键约束（指向 users 表的外键）
	// 背景：早期 AutoMigrate 未禁用外键时，会在 articles 库创建指向同库 users 表的外键，
	// 但 users 实际在 gin_project 库，导致插入/更新带 user_id 的行时报 1452 外键约束失败。
	// 现在 DisableForeignKeyConstraintWhenMigrating 已开启，不会再创建新外键，
	// 但已存在的外键需在此主动 DROP，否则会持续触发 1452 错误。
	dropCrossDBForeignKeys()

	// 注册 SkyWalking 链路追踪插件（自动拦截所有 SQL 查询）
	DB.Use(skywalking.GormPlugin())
	ArticleDB.Use(skywalking.GormPlugin())
}

// PingDB 检查数据库连接是否正常
func PingDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// PingArticleDB 检查文章库连接是否正常
func PingArticleDB() error {
	sqlDB, err := ArticleDB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// dropCrossDBForeignKeys 删除 articles 库中所有指向 users 表的外键约束
// 这些外键是早期 AutoMigrate 误建的跨库外键（users 实际在 gin_project 库），
// 会导致带 user_id 的 INSERT/UPDATE 触发 1452 外键约束失败
func dropCrossDBForeignKeys() {
	type fkInfo struct {
		TableName      string `gorm:"column:TABLE_NAME"`
		ConstraintName string `gorm:"column:CONSTRAINT_NAME"`
	}
	var fks []fkInfo
	// 查询 articles 库中所有指向 users 表的外键约束
	sql := `SELECT TABLE_NAME, CONSTRAINT_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME = 'users'`
	if err := ArticleDB.Raw(sql, conf.ArticleDBConfig.DBName).Scan(&fks).Error; err != nil {
		// 查询失败不阻断启动，仅记录
		return
	}
	for _, fk := range fks {
		// 逐个删除外键，忽略错误（外键可能已不存在）
		ArticleDB.Exec("ALTER TABLE `" + fk.TableName + "` DROP FOREIGN KEY `" + fk.ConstraintName + "`")
	}
}

// backfillCommentRootID 一次性回填存量评论的 root_id 和 reply_to_user_id
// 背景：这两个字段是后加的，AutoMigrate 只新增列（默认0），老回复的 root_id=0 会被当成顶级评论，
// 导致回复关系丢失、不显示 @被回复者。本函数在启动时自动修复，幂等（已回填的不会重复处理）。
//
// 逻辑：
//  1. 查所有 root_id=0 且 parent_id>0 的"未修复回复"
//  2. 对每条，沿 parent_id 链向上找根评论（root_id=0 且 parent_id=0 的顶级评论）
//  3. 设置 root_id=根评论ID，reply_to_user_id=父评论的 user_id
//  4. 若父评论链断裂（父被删），保持 root_id=0 作为孤儿顶级评论显示
func backfillCommentRootID() {
	// 查所有需要回填的回复（root_id=0 且有 parent_id）
	type rawComment struct {
		ID       uint `gorm:"column:id"`
		ParentID uint `gorm:"column:parent_id"`
	}
	var pending []rawComment
	if err := ArticleDB.Table("comments").
		Select("id, parent_id").
		Where("root_id = 0 AND parent_id > 0").
		Find(&pending).Error; err != nil {
		return
	}
	if len(pending) == 0 {
		return // 无需回填
	}

	// 构建 id → parent_id 映射（含所有评论，用于向上溯源）
	allComments := make([]rawComment, 0, len(pending)*2)
	ArticleDB.Table("comments").Select("id, parent_id").Find(&allComments)
	parentMap := make(map[uint]uint, len(allComments))
	for _, c := range allComments {
		parentMap[c.ID] = c.ParentID
	}

	// 逐条溯源找根评论
	type updateItem struct {
		ID           uint
		RootID       uint
		ReplyToUserID uint
	}
	var updates []updateItem
	for _, p := range pending {
		// 沿 parent_id 链向上找，最多 50 层防循环
		cur := p.ParentID
		rootID := uint(0)
		for i := 0; i < 50; i++ {
			pid, ok := parentMap[cur]
			if !ok {
				// 父评论不存在（已物理删除），链断裂，保持 root_id=0 作为孤儿
				break
			}
			if pid == 0 {
				// cur 就是顶级评论
				rootID = cur
				break
			}
			cur = pid
		}
		if rootID == 0 {
			// 找不到根评论，跳过（保持孤儿）
			continue
		}
		// reply_to_user_id = 直接父评论的 user_id
		var parentUserID uint
		ArticleDB.Table("comments").Select("user_id").Where("id = ?", p.ParentID).Scan(&parentUserID)
		updates = append(updates, updateItem{
			ID:            p.ID,
			RootID:        rootID,
			ReplyToUserID: parentUserID,
		})
	}

	// 批量更新（逐条 UPDATE，量小且幂等）
	for _, u := range updates {
		ArticleDB.Table("comments").
			Where("id = ?", u.ID).
			Updates(map[string]interface{}{
				"root_id":         u.RootID,
				"reply_to_user_id": u.ReplyToUserID,
			})
	}
}
