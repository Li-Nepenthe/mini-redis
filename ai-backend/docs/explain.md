# M1 SQL 与索引实测

MySQL 8.4.11；官方 mysql:8.4、独占临时数据库，本次实测为2026-10-02。

同一数据复制到独立对照表，实际删除/增加索引后各 EXPLAIN；不用猜测数字或改用户表。原表另执行同样关键 SQL。样本100 users、10,000 conversations、10,000 messages（加上功能测试自己的少量记录）。

| 查询 | 无索引 access/rows | 加索引 access/rows | 实际业务表 access/rows |
|---|---|---|---|
| users | ALL/104 | ref/1 | const/1 |
| conversations | ALL/9902 | ref/100 | ref/100 |
| messages | ALL/9824 | ref/1 | ref/1 |

三个业务 SQL：

```sql
SELECT id,email,password_hash,status,created_at FROM users WHERE email='ex-seed-42@example.test';
SELECT id,user_id,title,created_at,updated_at FROM conversations WHERE user_id='ex-user-42' ORDER BY created_at DESC,id DESC LIMIT 20;
SELECT id,role,content FROM messages WHERE conversation_id='ex-conv-42' ORDER BY created_at,id LIMIT 20;
```

原始 FORMAT=JSON 的提取字段保存在本任务 work/validation/p2-m1-explain.json；rows 是优化器估计行数，不是实际扫描计时。小样本与统计信息会改变估值。对照表已删除；应用表索引未改。
