/* 把 gin_project 库下 article 相关表数据导入 articles 库 */
/* 关闭外键检查后即可绕过跨库外键约束（articles 库的 users 表为空） */

SET FOREIGN_KEY_CHECKS = 0;

TRUNCATE TABLE articles.article_tags;
TRUNCATE TABLE articles.comments;
TRUNCATE TABLE articles.article_edit_approvals;
TRUNCATE TABLE articles.articles;
TRUNCATE TABLE articles.tags;

INSERT INTO articles.articles SELECT * FROM gin_project.articles;
INSERT INTO articles.tags SELECT * FROM gin_project.tags;
INSERT INTO articles.article_tags SELECT * FROM gin_project.article_tags;
INSERT INTO articles.article_edit_approvals SELECT * FROM gin_project.article_edit_approvals;
INSERT INTO articles.comments SELECT * FROM gin_project.comments;

SET FOREIGN_KEY_CHECKS = 1;

SELECT
    (SELECT COUNT(*) FROM articles.articles) AS articles_count,
    (SELECT COUNT(*) FROM articles.tags) AS tags_count,
    (SELECT COUNT(*) FROM articles.article_tags) AS article_tags_count,
    (SELECT COUNT(*) FROM articles.article_edit_approvals) AS approvals_count,
    (SELECT COUNT(*) FROM articles.comments) AS comments_count;
