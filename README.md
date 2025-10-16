# Daily LeetCode Discord Bot

Bot viết bằng Go theo clean architecture, mỗi ngày sẽ:
- Lấy **daily challenge** từ LeetCode (kèm mô tả đã convert sang text).
- Lấy ngẫu nhiên một vài bài LeetCode để luyện thêm (loại trừ bài trùng với daily).
- Chọn bài đọc thuật toán từ Medium và dev.to/tag/algorithms.
- Nhờ Gemini Flash (mặc định) viết ghi chú hằng ngày theo phong cách giáo sư thuật toán, giải thích lý do và trích dẫn từ *Grokking Algorithms*.
- Gửi toàn bộ vào Discord thông qua webhook với embedded message.

## Cấu trúc chính

- `cmd/bot/main.go`: entrypoint.
- `internal/app`: scheduler và lifecycle.
- `internal/usecase`: nghiệp vụ tổng hợp dữ liệu + tạo thông báo.
- `internal/domain`: model và port (interface) theo clean architecture.
- `internal/adapter`: các adapter cho LeetCode, Medium/dev.to article nguồn, ghi chú Gemini, Discord webhook, logging.
- `internal/config`: đọc config từ biến môi trường.
- `internal/di`: wire DI graph.

## Yêu cầu

- Go >= 1.22.
- Cài công cụ `wire` (được cài tự động khi chạy `go install github.com/google/wire/cmd/wire@latest`).
- Discord webhook URL hợp lệ.

## Thiết lập

Tạo file `.env` trong thư mục gốc:

```bash
# Copy file mẫu
cp .env.example .env

# Hoặc tạo thủ công
cat > .env << 'EOF'
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_URL
GEMINI_API_KEY=YOUR_GEMINI_API_KEY
DISCORD_BOT_TOKEN=YOUR_BOT_TOKEN
EOF
```

**Các biến môi trường cần thiết:**
- `DISCORD_WEBHOOK_URL`: Discord webhook URL (bắt buộc)
- `GEMINI_API_KEY`: Google Gemini API key (bắt buộc)
- `DISCORD_BOT_TOKEN`: Discord bot token (tùy chọn)
**Biến môi trường tùy chọn:**
- `GEMINI_MODEL`: Model Gemini (mặc định: gemini-2.5-flash)
- `GEMINI_TOPIC_LIMIT`: Giới hạn số topics (mặc định: 3)
- `SCHEDULE_CRON`: Cron schedule (mặc định: "0 9 * * *")
- `RANDOM_PROBLEM_COUNT`: Số bài random LeetCode (mặc định: 2)
- `ARTICLE_COUNT`: Số bài đọc (mặc định: 2)
- `REQUEST_TIMEOUT`: HTTP timeout (mặc định: 30s)
```

## Chạy bot

```bash
go run ./cmd/bot
```

Bot sẽ:
1. Gửi digest ngay khi khởi động (giúp test nhanh).
2. Chạy theo lịch `SCHEDULE_CRON` với cron chuẩn (không có giây).
3. Lắng nghe tín hiệu `SIGINT`/`SIGTERM` để shutdown gọn.

Để chạy background, có thể:
- Dùng `screen`, `tmux` hoặc systemd service.
- Hoặc build binary: `go build -o bin/daily-bot ./cmd/bot`.

## Phát triển thêm

- Source LeetCode dùng API công khai (`/graphql` & `/api/problems/all/`). Nếu cần account / cookie riêng, có thể mở rộng `internal/adapter/leetcode`.
- Module bài viết hiện lấy từ Medium + dev.to; có thể thêm nguồn khác (Hacker News, YouTube playlist, v.v) bằng cách implement `ports.ArticleProvider` và bổ sung vào composite.
- Ghi chú học thuật được sinh bởi Gemini; có thể thay prompt hoặc thêm writer khác bằng cách implement `ports.ArticleWriter`.
- Notifier hiện là Discord webhook; có thể thêm Slack, email… bằng cách implement `ports.Notifier`.
