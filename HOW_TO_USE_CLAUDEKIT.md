# HOW TO USE CLAUDEKIT

---

## 1. ClaudeKit Engineer là gì?

Đây là một boilerplate/framework cho Claude Code, cung cấp:

- 80+ skills (lệnh slash `/ck:...`)
- 14 agents chuyên biệt (planner, tester, reviewer, debugger...)
- 15 hooks tự động hóa (session init, privacy block, code quality...)
- 6 coding-level profiles (từ ELI5 đến God-tier)
- Quy trình phát triển có cấu trúc: **Plan → Cook → Test → Review → Docs**

---

## 2. Cách cài đặt thủ công (KHÔNG cần `ck init`)

### Điều kiện tiên quyết

- Claude Code đã cài đặt (`npm install -g @anthropic-ai/claude-code`)
- Node.js 18+
- Git

### Các bước sao chép vào dự án của bạn

Bạn cần copy 4 thứ từ thư mục `claudekit-engineer` vào dự án đích:

```
Từ:      D:\Project\claudekit-engineer\
Sao chép vào: D:\Project\your-project\
```

| #   | Thứ cần copy | Ghi chú                                              |
| --- | ------------ | ---------------------------------------------------- |
| 1   | `.claude/`   | **THƯ MỤC CHÍNH (bắt buộc)** — chứa toàn bộ hệ thống |
| 2   | `CLAUDE.md`  | File hướng dẫn ở root (bắt buộc)                     |
| 3   | `AGENTS.md`  | File cho OpenCode agents (tùy chọn)                  |
| 4   | `plans/`     | Thư mục quản lý kế hoạch (nên copy)                  |
| 5   | `docs/`      | Template docs (nên copy, sau đó chỉnh sửa cho dự án) |

### Lệnh copy cụ thể (chạy trong Git Bash / terminal)

```bash
# Di chuyển đến dự án đích
cd /d/Project/your-project

# Copy thư mục .claude (QUAN TRỌNG NHẤT)
cp -r /d/Project/claudekit-engineer/.claude .

# Copy CLAUDE.md
cp /d/Project/claudekit-engineer/CLAUDE.md .

# Copy AGENTS.md (tùy chọn, cho OpenCode)
cp /d/Project/claudekit-engineer/AGENTS.md .

# Copy plans/ (cấu trúc quản lý kế hoạch)
cp -r /d/Project/claudekit-engineer/plans .

# Copy docs/ (template tài liệu - chỉnh sửa lại cho dự án của bạn)
cp -r /d/Project/claudekit-engineer/docs .
```

### Sau khi copy, cấu hình thêm

#### a) Tạo file `.claude/.env` (API keys)

```bash
cp .claude/.env.example .claude/.env
```

Mở `.claude/.env` và điền các key cần thiết:

```env
# Quan trọng nhất - cho các skill AI
GEMINI_API_KEY=your_gemini_key_here

# Tùy chọn - thông báo
DISCORD_WEBHOOK_URL=
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=

# Tùy chọn - các AI khác
# OPENAI_API_KEY=
# ANTHROPIC_API_KEY=
```

#### b) Cấu hình MCP servers (tùy chọn nhưng rất hữu ích)

```bash
cp .claude/.mcp.json.example .claude/.mcp.json
```

Chỉnh sửa `.claude/.mcp.json` và thay `YOUR_API_KEY` bằng key thật:

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp", "--api-key", "YOUR_CONTEXT7_KEY"]
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"]
    }
  }
}
```

> **Lưu ý:** `sequential-thinking` không cần API key, có thể dùng ngay.

#### c) Chỉnh sửa `CLAUDE.md` cho dự án của bạn

Mở `CLAUDE.md` ở root và tùy chỉnh phần mô tả dự án, kiến trúc, quy ước... phù hợp với dự án thực tế của bạn.

---

## 3. Khởi động và sử dụng

```bash
cd /d/Project/your-project
claude
```

Khi Claude Code khởi động, hệ thống hooks sẽ tự động:

1. Phát hiện loại dự án (Node.js, Python, Go, Rust...)
2. Phát hiện framework (React, Next.js, FastAPI, Django...)
3. Phát hiện package manager (npm, yarn, pip...)
4. Load cấu hình từ `.ck.json`
5. Hiển thị statusline với thông tin dự án

---

## 4. Các Skills chính (Slash Commands)

### Nhóm Lập kế hoạch & Triển khai

| Lệnh                         | Mô tả                                 |
| ---------------------------- | ------------------------------------- |
| `/ck:plan "mô tả tính năng"` | Tạo kế hoạch triển khai chi tiết      |
| `/ck:cook "mô tả tính năng"` | Triển khai tính năng (tự plan + code) |
| `/ck:fix "mô tả bug"`        | Sửa bug có hệ thống                   |
| `/ck:debug "mô tả vấn đề"`   | Debug & phân tích nguyên nhân gốc     |

### Nhóm Chất lượng code

| Lệnh                | Mô tả                      |
| ------------------- | -------------------------- |
| `/ck:test`          | Chạy test suite            |
| `/ck:code-review`   | Review code tự động        |
| `/ck:simplify`      | Đơn giản hóa code vừa viết |
| `/ck:security-scan` | Quét lỗ hổng bảo mật       |

### Nhóm Nghiên cứu & Tư vấn

| Lệnh                         | Mô tả                          |
| ---------------------------- | ------------------------------ |
| `/ck:ask "câu hỏi"`          | Hỏi chuyên gia kỹ thuật        |
| `/ck:research "chủ đề"`      | Nghiên cứu sâu một công nghệ   |
| `/ck:brainstorm "ý tưởng"`   | Brainstorm giải pháp           |
| `/ck:docs-seeker "thư viện"` | Tra cứu docs thư viện mới nhất |

### Nhóm Git & Ship

| Lệnh       | Mô tả                                        |
| ---------- | -------------------------------------------- |
| `/ck:git`  | Quản lý git (commit, push...)                |
| `/ck:ship` | Pipeline: test → review → commit → push → PR |

### Nhóm Docs & Quản lý

| Lệnh          | Mô tả                         |
| ------------- | ----------------------------- |
| `/ck:docs`    | Cập nhật tài liệu dự án       |
| `/ck:watzup`  | Xem trạng thái dự án hiện tại |
| `/ck:retro`   | Retrospective sprint          |
| `/ck:journal` | Ghi nhật ký phát triển        |

### Nhóm Frontend & Design

| Lệnh                  | Mô tả                                 |
| --------------------- | ------------------------------------- |
| `/ck:frontend-design` | Tạo giao diện từ mô tả/screenshot     |
| `/ck:ui-styling`      | Style với shadcn/ui + Tailwind        |
| `/ck:design`          | Brand identity, logo, design tokens   |
| `/ck:stitch`          | Generate UI design bằng Google Stitch |

### Nhóm AI & Multimedia

| Lệnh                | Mô tả                                            |
| ------------------- | ------------------------------------------------ |
| `/ck:ai-multimodal` | Phân tích/tạo ảnh, video, audio (cần Gemini key) |
| `/ck:ai-artist`     | Tạo ảnh AI với 129 prompt styles                 |

### Nhóm DevOps & Deploy

| Lệnh            | Mô tả                                       |
| --------------- | ------------------------------------------- |
| `/ck:deploy`    | Deploy lên Vercel/Netlify/Railway/Fly.io... |
| `/ck:devops`    | Docker, K8s, CI/CD                          |
| `/ck:bootstrap` | Khởi tạo dự án mới hoàn chỉnh               |

---

## 5. Quy trình phát triển chuẩn

### Phát triển tính năng mới

```
Bước 1: /ck:plan "implement user authentication with OAuth2"
         → Tạo kế hoạch chi tiết trong plans/

Bước 2: /ck:cook
         → Triển khai theo kế hoạch

Bước 3: (tự động) Simplify code sau mỗi lần edit

Bước 4: /ck:test
         → Chạy test, đảm bảo pass

Bước 5: /ck:code-review
         → Review chất lượng code

Bước 6: /ck:docs
         → Cập nhật tài liệu

Bước 7: /ck:ship
         → Commit → Push → tạo PR
```

### Sửa bug

```
/ck:debug "login fails with 500 error"    → Phân tích
/ck:fix "authentication null pointer"     → Sửa
/ck:test                                  → Xác nhận
```

---

## 6. Hệ thống Agents (14 agents chuyên biệt)

Claude Code sẽ tự động gọi các agent phù hợp khi bạn dùng skill. Bạn cũng có thể yêu cầu trực tiếp:

| Agent                 | Vai trò                                      |
| --------------------- | -------------------------------------------- |
| `planner`             | Lập kế hoạch triển khai, phân tích kiến trúc |
| `researcher`          | Nghiên cứu công nghệ, best practices         |
| `fullstack-developer` | Code implementation                          |
| `tester`              | Viết & chạy test                             |
| `code-reviewer`       | Review code chất lượng                       |
| `code-simplifier`     | Tối ưu & đơn giản hóa code                   |
| `debugger`            | Debug & root cause analysis                  |
| `docs-manager`        | Quản lý documentation                        |
| `git-manager`         | Git workflow                                 |
| `project-manager`     | Theo dõi tiến độ dự án                       |
| `ui-ux-designer`      | Thiết kế giao diện                           |
| `brainstormer`        | Brainstorm giải pháp                         |
| `journal-writer`      | Ghi chép nhật ký                             |
| `mcp-manager`         | Quản lý MCP servers                          |

---

## 7. Hệ thống Hooks (tự động hóa)

Hooks chạy tự động ở background, bạn không cần can thiệp:

| Hook                      | Thời điểm            | Tác dụng                                   |
| ------------------------- | -------------------- | ------------------------------------------ |
| `session-init`            | Mở Claude Code       | Auto-detect dự án, framework, PM           |
| `session-state`           | Start/Stop           | Lưu trạng thái session                     |
| `dev-rules-reminder`      | Mỗi prompt           | Nhắc tuân thủ quy tắc dev                  |
| `usage-context-awareness` | Mỗi prompt + edit    | Theo dõi token usage                       |
| `scout-block`             | Trước Read/Glob/Grep | Chặn đọc file lớn không cần thiết          |
| `privacy-block`           | Trước Read/Edit      | Bảo vệ file nhạy cảm (`.env`, credentials) |
| `descriptive-name`        | Trước Write          | Kiểm tra file name có descriptive không    |
| `post-edit-simplify`      | Sau Edit/Write       | Nhắc simplify code                         |
| `plan-format-kanban`      | Sau edit plan        | Format plan dạng kanban                    |

---

## 8. Cấu hình quan trọng (`.claude/.ck.json`)

```json
{
  "codingLevel": -1, // -1 = auto-detect, 0-5 = thủ công
  "statusline": "full", // "full" | "compact" | "off"
  "privacyBlock": true, // Bảo vệ file nhạy cảm
  "docs": { "maxLoc": 800 }, // Giới hạn dòng code trong docs
  "plan": {
    "namingFormat": "{date}-{issue}-{slug}",
    "validation": {
      "mode": "prompt", // Hỏi trước khi triển khai
      "minQuestions": 3,
      "maxQuestions": 8
    }
  },
  "project": {
    "type": "auto", // Hoặc "nodejs", "python", "go"...
    "packageManager": "auto",
    "framework": "auto"
  },
  "gemini": {
    "model": "gemini-3-flash-preview"
  }
}
```

Bạn có thể chỉnh:

- **`codingLevel`**: Đặt `0` (beginner) → `5` (expert) để thay đổi độ chi tiết phản hồi
- **`privacyBlock`**: `false` nếu không cần bảo vệ file nhạy cảm
- **`project.type`**: Đặt cứng nếu auto-detect sai

---

## 9. Coding Level (6 mức)

Thay đổi bằng `/coding-level`:

| Level        | Phong cách                                     | Phù hợp           |
| ------------ | ---------------------------------------------- | ----------------- |
| `0 - ELI5`   | Giải thích cực kỳ chi tiết, ví dụ nhiều        | Người mới bắt đầu |
| `1 - Junior` | Hướng dẫn từng bước, giải thích best practices | Junior dev        |
| `2 - Mid`    | Cân bằng chi tiết & ngắn gọn                   | Mid-level dev     |
| `3 - Senior` | Focus kiến trúc, trade-offs                    | Senior dev        |
| `4 - Lead`   | Chiến lược, scalability                        | Tech lead         |
| `5 - God`    | Cực kỳ ngắn gọn, advanced patterns             | Expert            |

---

## 10. Tổng kết: Checklist sao chép hoàn chỉnh

**Những thứ cần copy:**

- ✅ Copy `.claude/` → Toàn bộ hệ thống (hooks, skills, agents, rules)
- ✅ Copy `CLAUDE.md` → Hướng dẫn cho Claude Code
- ✅ Copy `plans/` → Cấu trúc quản lý kế hoạch
- ✅ Copy `docs/` → Template tài liệu (chỉnh sửa lại)
- ⬜ Copy `AGENTS.md` → Chỉ cần nếu dùng OpenCode
- ⬜ Copy `.commitlintrc.json` → Chỉ cần nếu muốn lint commit messages
- ⬜ Copy `.releaserc.cjs` → Chỉ cần nếu muốn semantic-release

**Sau khi copy:**

- ✅ `cp .claude/.env.example .claude/.env` → Điền API keys
- ✅ `cp .claude/.mcp.json.example .claude/.mcp.json` → Cấu hình MCP (tùy chọn)
- ✅ Chỉnh `CLAUDE.md` phù hợp dự án thực tế
- ✅ Chỉnh `docs/` phù hợp dự án thực tế

---

> Sau đó chỉ cần `claude` trong thư mục dự án là dùng được toàn bộ tính năng.
> Không cần `ck init`, không cần tài khoản ClaudeKit.
> Bản chất ClaudeKit chỉ là một bộ file cấu hình cho Claude Code — **copy vào là chạy.**
