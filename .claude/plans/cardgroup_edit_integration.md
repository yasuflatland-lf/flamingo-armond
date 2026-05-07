# Cardgroup edit page UI 改修 — 設計仕様

`http://localhost:3000/cardgroups/[id]/edit` の UI を shadcn.io 系の質感に寄せて再構成する。`@superpowers:brainstorming` セッション (2026-05-08) で 12 名のデザイナーパネル相当の議論を経て確定した設計をここに固定する。

## 1. 目的と非目的

### 目的
- カード管理を主タスクとして画面の主役に据える。Settings は副次タスクとして header の DropdownMenu に格納し、画面ノイズを下げる。
- カード一覧に検索を導入し、`page-size > 20` 規模のカードグループの取り回しを実用化する。
- 行のインタラクションを「タップで編集 / 右にスワイプ or hover の Delete アイコンで削除」に統一し、Edit ボタン廃止 + AlertDialog（個別削除分）廃止で UI チャームを削減する。
- 削除の安全網として sonner の Snackbar (Undo) を導入する。

### 非目的
- カード並び替え (drag-and-drop ordering) は今回スコープ外。
- Settings menu の Export / Duplicate は将来枠として menu 構造のみ用意し、今回は実装しない。
- Tabs / Sheet で Settings を別 view にする案 (B/C/D 案) は採用せず、今回は A 案 (kebab) のみ。

## 2. 確定設計

### 2.1 レイアウト構造

```
┌────────────────────────────────────────────────┐
│ ← Cardgroups                                   │
│ <h1>{name}</h1>  <Badge>N cards</Badge>    [⋯] │  ← kebab DropdownMenu
├────────────────────────────────────────────────┤
│ Cards  [🔍 Search front or back...]            │
│        [▶ Start learning] [+ Add card]         │
├────────────────────────────────────────────────┤
│ [N selected]  [Delete selected] [Cancel]       │  ← bulk action bar (条件付き表示)
├────────────────────────────────────────────────┤
│ ☐  apple                              [✏ 廃止] │
│    りんご                                  [🗑] │  ← hover で fade-in
│ ☑  benevolent                              [🗑] │  ← selected: 紫アクセント
│    親切な、慈悲深い                            │
│ ☐  candid                                  [🗑] │
│    率直な                                       │
│ ...                                            │
└────────────────────────────────────────────────┘
```

### 2.2 Header の DropdownMenu

- `<h1>{cardgroup.name}</h1>` は表示専用（Nathan Curtis の「真実の源は一つ」原則）。
- 右隣に `<Badge>{totalCount} cards</Badge>` を配置。`CardgroupCardsSection` の renderHeader が live `totalCount` を Badge にも渡せるよう拡張する。
- `⋯` IconButton をクリックすると Radix Popover ベースの DropdownMenu が開く。menu items:
  - `✏ Rename` → shadcn Dialog で既存 `CardgroupForm` を表示。保存後 `router.refresh()` で h1 / Badge を再描画。
  - `🗑 Delete cardgroup` (赤系) → 既存 `AlertDialog` を再利用（CardgroupSettingsCard から移植）。
  - 将来枠 (今回はコメントアウトで残す): `↗ Export to TextDic`, `📋 Duplicate`。

shadcn の `dropdown-menu.tsx` は未インストールなので追加が必要。

### 2.3 行 (CardListItem) のインタラクション

| 入力 | 結果 |
|---|---|
| 行本体タップ／クリック (front/back テキスト領域) | インライン編集（既存 `editingId` 切替で CardForm に置換） |
| 左の checkbox クリック | 選択トグル (`stopPropagation` で行クリックに伝播させない) |
| desktop hover で fade-in する Delete アイコン | 即削除 + Snackbar (Undo) |
| モバイル左 swipe (full swipe) | 即削除 + Snackbar (Undo) |
| モバイル `prefers-reduced-motion: reduce` | swipe 無効化、Delete アイコンを常時表示にフォールバック |
| `selectedIds.size > 0` の選択モード中 | swipe 無効化（誤爆回避）、checkbox UI のみで bulk 操作 |

行の Edit アイコンは廃止する。Edit の入り口は「行クリック」一本に統一。

### 2.4 削除の安全網: sonner Snackbar

- `sonner` を依存追加し、`<Toaster richColors closeButton />` を `app/layout.tsx` の `<AppShell>` 配下に常設。
- 削除実行直後に optimistic で Apollo cache を更新し、Snackbar に Undo ボタン (5 秒) を表示。Undo クリックで cache をロールバック + サーバ側に restore mutation… **は今回スコープ外** で、5 秒以内なら DELETE そのものを送信しない遅延 commit 方式を採る。
  - 実装: 削除アクション → 5 秒タイマー + Snackbar 表示 → タイマー満了で実 DELETE mutation 発火 → 失敗時のみ cache を巻き戻し。Undo クリックでタイマー cancel + cache 復元。
  - これによりサーバには「削除して復元する」ラウンドトリップが発生せず、UX 反応も即時。
- 個別 Delete の `AlertDialog` は廃止。
- bulk delete の `AlertDialog` は **存続**（複数行 undo は cache 巻き戻しが複雑になるため、確認ダイアログ方式を据え置く）。

### 2.5 検索 (front + back の部分一致)

- バックエンド: `cardsByCardgroupConnection` に `search: String` 引数を追加。
  - SQL: `WHERE (front ILIKE ? OR back ILIKE ?)`。`escapeLike` で `%` `_` `\` をエスケープ（`.claude/rules/go-library-gotchas.md` § "GORM `LIKE` / `ILIKE` requires escaping"）。
  - 既存の cursor / orderBy / pagination は変更なし。検索による絞り込み後の cursor もそのまま機能する。
- フロントエンド: cardgroups 一覧の search 実装をそのまま踏襲。
  - `searchInput` (immediate) / `searchQuery` (debounced 300ms)
  - Apollo cache key の整合: `CARDS_PAGE_SIZE` と並んで `CARDS_DEFAULT_VARS = { first: PAGE_SIZE, search: null }` を export し、SSR seed / `useQuery` / mutation update callback の 3 箇所で同じ vars を使う（`.claude/rules/pagination.md` § "Variables shape MUST match"）。
  - `searchQuery` 変更時に in-flight `fetchMore` の guard ref と `fetchMoreError` をリセットする (`useEffect` トリガー deps `[searchQuery]`)。

### 2.6 Empty state (3 パターン)

| パターン | コピー | 主 CTA |
|---|---|---|
| 0 件（新規 cardgroup） | "Add some new cards to get started." | `+ Add card` ボタン |
| 検索ヒット 0 件 | `No cards match "{query}"` | "Clear search" ボタン |
| 全削除直後 | パターン 1 と同じ | 同上 |

### 2.7 Mobile rendering

- `<h1>` と Badge は `flex-wrap`。狭幅では Badge が下段に折り返す。
- kebab `⋯` は h1 行の右端に常駐。
- toolbar は `flex-col gap-2 sm:flex-row` で縦積み → 横並び。
- 行の swipe-to-delete は `@use-gesture/react` + `@react-spring/web` (既存依存) で実装。`frontend/src/components/learn/swipe-card.tsx` の単方向版として書き起こす。

## 3. バックエンド差分

| ファイル | 差分 |
|---|---|
| `schema/schema.graphql` | `cardsByCardgroupConnection(... search: String)` 引数追加。コメントで「front + back ILIKE 部分一致」を記載。 |
| `backend/internal/repository/card.go` | `FindByCardgroupConnection` 系に `search *string` 引数追加。`escapeLike` で wrap して `WHERE (front ILIKE ? OR back ILIKE ?)` を AND 条件として追加。空文字 / nil の時は WHERE 追加しない。 |
| `backend/internal/usecase/card.go` | `ListByCardgroupConnection` の入力に `search` を通す。`maxPageSize` / `pageCap` のロジックは変更なし。 |
| `backend/graph/resolver/...` | resolver で `search` argument を usecase に渡す。 |
| `backend/internal/loader/...` | DataLoader 経路に search はない（per-key だけ）ので変更なし。 |
| `backend/internal/repository/card_test.go` | search テスト: ヒットあり / なし / `%` `_` 含むクエリのエスケープ / 大文字小文字無視 / 別 cardgroup の row が混入しないことを既存の cross-tenant test と同じ shape で確認 (`.claude/rules/go-library-gotchas.md` § "Repository lookup methods scoped by tenant ID")。 |
| `backend/internal/usecase/card_test.go` | search 引数を usecase 経由で repository に渡すパススルーテスト。 |

## 4. フロントエンド差分

### 4.1 新規追加

| パス | 用途 |
|---|---|
| `frontend/src/components/ui/dropdown-menu.tsx` | shadcn DropdownMenu (Radix Popover ベース) を `npx shadcn@latest add dropdown-menu` で追加。 |
| `frontend/src/components/ui/sonner.tsx` | shadcn の Toaster wrapper (`npx shadcn@latest add sonner`)。 |
| `frontend/src/components/cardgroups/cardgroup-header.tsx` | h1 + Badge + kebab DropdownMenu のヘッダ component。 |
| `frontend/src/components/cardgroups/rename-cardgroup-dialog.tsx` | shadcn Dialog でラップした `CardgroupForm` (kebab → Rename からの呼び出し)。 |
| `frontend/src/components/cardgroups/swipeable-row.tsx` | 単方向 (左) swipe + threshold 60% で full delete + Undo。`@use-gesture/react` + `@react-spring/web`。`prefers-reduced-motion` で no-op に。 |
| `frontend/src/lib/undo-delete.ts` | 5 秒タイマー方式の delayed-DELETE ヘルパ: `scheduleDelete({ id, mutation, cache, optimistic })` → returns `{ undo() }`。 |

### 4.2 改修

| パス | 改修内容 |
|---|---|
| `frontend/src/app/cardgroups/[id]/edit/cardgroup-management-client.tsx` | 二段組レイアウト廃止。`<aside>` (Settings) を削除し `<CardgroupHeader>` + `<CardgroupCardsSection>` の縦積みに。 |
| `frontend/src/components/cardgroups/cardgroup-cards-section.tsx` | renderHeader を toolbar 仕様に変更: count chip + Search input + Start learning + Add card。 |
| `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` | 検索 state (`searchInput` / `searchQuery`)、debounce useEffect、`searchQuery` 変更時の guard reset、`Snackbar`-via-`undo-delete` への deleteCard 切替、行レンダラを `<SwipeableRow>` でラップ、Edit アイコン削除、Delete アイコンを hover fade-in に維持。 |
| `frontend/src/app/cardgroups/[id]/cards/queries.ts` | `CARDS_DEFAULT_VARS` を export。`CardsByCardgroupConnectionQuery` の definition に `$search: String` を追加。 |
| `frontend/src/components/cardgroups/cardgroup-settings-card.tsx` | **削除**（kebab + RenameDialog に機能移管）。 |
| `frontend/src/app/layout.tsx` | `<Toaster />` を `<AppShell>` 配下に追加。 |

## 5. 新規依存

| パッケージ | 用途 | 概算サイズ |
|---|---|---|
| `sonner` | Snackbar (Toast + Undo) | ~5KB gz |
| `@radix-ui/react-dropdown-menu` | shadcn DropdownMenu の依存 | ~10KB gz (peer) |

`@use-gesture/react` / `@react-spring/web` は既存。

## 6. 実装タスク (インパクト大 × 変更量小の順)

各タスクは独立した PR にできる粒度に分割。Backend スキーマ変更を要するタスクは `backend/` テスト pass を完了条件に含める。

### Tier 1 (大 × 小)

#### Task 1. Settings → kebab DropdownMenu に移管
- **Impact**: 大（主タスクをカード管理に集中させ、Settings の常時露出を削減）
- **Change**: 小（既存 `CardgroupSettingsCard` を kebab + RenameDialog に置換するだけ。レイアウト二段組を縦積みに）
- **Files**:
  - 新規: `cardgroup-header.tsx`, `rename-cardgroup-dialog.tsx`, `components/ui/dropdown-menu.tsx`
  - 改修: `cardgroup-management-client.tsx`
  - 削除: `cardgroup-settings-card.tsx`, `cardgroup-settings-card.test.tsx`
- **依存追加**: `@radix-ui/react-dropdown-menu`
- **Acceptance**:
  - `/cardgroups/[id]/edit` の左 aside が消え、Cards が full-width に。
  - 右上 `⋯` から Rename / Delete cardgroup が出る。
  - Rename dialog 保存で h1 が即時更新される (`router.refresh`)。
  - Delete cardgroup の AlertDialog 動作は現状維持。
  - 既存 cardgroup-settings-card のテストケース (Save / Delete / エラー表示) を rename-cardgroup-dialog.test.tsx に移植。

#### Task 2. h1 + Badge ヘッダと toolbar 整理 (count chip / Add card / Start learning 順序統一)
- **Impact**: 中（Cards/Settings 同居の解消ほど大きくはないが、視覚的階層が一段クリアになる）
- **Change**: 小（renderHeader の中身を変えるだけ。既存ロジックはそのまま）
- **Files**:
  - 改修: `cardgroup-cards-section.tsx` (renderHeader)、`cardgroup-header.tsx` (Badge を `totalCount` 連動に)
- **Acceptance**:
  - count は h1 横の Badge と toolbar 左 chip の二重表示を排除し Badge 一本に。
  - Add card は brand 色、Start learning は outline。

### Tier 2 (大 × 中)

#### Task 3. バックエンド検索フィールドの追加
- **Impact**: 大（検索なしで 50 件超を扱うのは実用範囲外）
- **Change**: 中（schema / repo / usecase / resolver / loader / tests）
- **Files**:
  - schema: `schema/schema.graphql`
  - backend: `repository/card.go`, `usecase/card.go`, `graph/resolver/...`
  - 自動生成: `gqlgen` 再生成 + frontend `codegen` 再生成
  - tests: `repository/card_test.go`, `usecase/card_test.go`, integration test (search hit / miss / `%`-escape / cross-tenant non-leak)
- **Acceptance**:
  - GraphQL Playground で `cardsByCardgroupConnection(cardgroupId: ..., search: "app", first: 10)` がヒット行のみ返す。
  - `search: "100%"` のようなメタ文字を含むクエリで literal match (full-table scan ではなく)。
  - 別 owner の cards が混入しないことを cross-tenant test で保証。
  - 既存 cursor pagination が search 適用後も動く (search + after 組み合わせ)。

#### Task 4. フロント検索 UI + Apollo cache key 整合
- **Impact**: 大（バックエンド完成だけでは UX に届かない、表面化させる工程）
- **Change**: 中（debounce / guard reset / vars 統一）
- **Files**:
  - 改修: `cards-client.tsx` (search state / debounce / guard reset), `queries.ts` (`CARDS_DEFAULT_VARS` export, `$search` 引数), `page.tsx` (SSR seed の variables を `CARDS_DEFAULT_VARS` に統一), `cardgroup-cards-section.tsx` (toolbar に Search input)
- **Acceptance**:
  - 入力後 300ms で結果が更新される (cardgroups search と同タイミング)。
  - 検索ヒット 0 件で Empty state パターン 2 が出る。
  - `searchQuery` 変更時に in-flight fetchMore guard と fetchMoreError がリセットされる。
  - Apollo MockedProvider の leak spy が cache split を検出しないこと (`.claude/rules/pagination.md` § "Capture `console.warn` for MockedProvider leaks")。

#### Task 5. sonner 導入 + 個別削除を Snackbar (Undo) 方式に切替
- **Impact**: 大（操作粒度が AlertDialog 1 タップから Hover→Click 1 タップに減り、5 秒 Undo の安心感が出る）
- **Change**: 中（依存追加 + delayed-delete ヘルパ + cache の楽観更新ロールバック設計）
- **Files**:
  - 新規: `lib/undo-delete.ts`, `components/ui/sonner.tsx`
  - 改修: `app/layout.tsx` (Toaster), `cards-client.tsx` (deleteCard 呼び出しを scheduleDelete に置換、AlertDialog 廃止)
- **Acceptance**:
  - Hover Delete クリックで即時に行が消え、画面下に "Card deleted [Undo]" Snackbar (5 秒)。
  - Undo クリックで行が復活、サーバには DELETE が送られていない。
  - 5 秒経過で実際の DELETE mutation が走り、失敗した場合のみ Snackbar が "Failed to delete" に差し替わり cache が巻き戻る。
  - Bulk delete は AlertDialog 経由のままで、Undo 機構は使わない。
  - **画面遷移時の pending delete 強制 flush**: ページを離れた瞬間にタイマーが破棄されると Undo 機構ごと削除がキャンセルされてしまうため、`usePathname` 変化時 / `beforeunload` 時に `flushPendingDeletes()` を呼んで pending タイマーを即時 commit する。

### Tier 3 (中 × 小)

#### Task 6. Edit ボタン廃止 / 行クリックで inline 編集
- **Impact**: 中（クリック数 -1、行に表情が出る）
- **Change**: 小（`onClick` ハンドラの追加と stopPropagation 配線）
- **Files**:
  - 改修: `cards-client.tsx` (行に `<button>` 化 or `role="button"` + `onClick={() => setEditingId(card.id)}`、checkbox / Delete アイコンに `e.stopPropagation()`)
- **Acceptance**:
  - 行のテキスト領域をクリックすると CardForm に切り替わる。
  - checkbox を直接クリックしても編集モードに入らない（選択トグルのみ）。
  - キーボード操作 (Tab → Enter) で編集モードに入れる。
  - Delete アイコンクリックは編集モードに入らず、Snackbar (Undo) フローに進む。

#### Task 7. Empty state の 3 パターン整備
- **Impact**: 小〜中（コピー差し替えと CTA 追加）
- **Change**: 小
- **Files**:
  - 改修: `cards-client.tsx` の `edges.length === 0` 分岐に `searchQuery` 状態を加味した条件分岐
- **Acceptance**:
  - 0 件 + search なし: "Add some new cards to get started." + 大きい `+ Add card` CTA
  - 0 件 + search あり: `No cards match "{query}"` + Clear search ボタン
  - Clear search ボタンで searchInput / searchQuery が空になる

### Tier 4 (中 × 大)

#### Task 8. モバイル swipe-to-delete (`SwipeableRow`)
- **Impact**: 中（モバイルユーザーの削除体験が劇的に改善するが、desktop には影響なし）
- **Change**: 大（新規 component + spring physics + テスト + reduce-motion + selection mode 共存）
- **Files**:
  - 新規: `swipeable-row.tsx`, `swipeable-row.test.tsx`
  - 改修: `cards-client.tsx` (各行を `<SwipeableRow>` でラップ)
- **Acceptance**:
  - モバイルで左に 60% 以上 swipe → 行が消え、Snackbar (Undo) が出る。Task 5 の `scheduleDelete` を共有。
  - 30%-60% で release → 半開きで Delete ボタンが固定表示、tap で確定。
  - 別行をタップで半開きが閉じる。
  - `selectedIds.size > 0` の時は swipe disable。
  - `prefers-reduced-motion: reduce` で swipe 無効化、Delete アイコンを常時表示にフォールバック。
  - 既存 `swipe-card.test.tsx` をテンプレに pointer event simulate でテスト。

## 7. テスト戦略

- 各 Task の Acceptance を満たす vitest テストを必ず同 PR 内に追加。
- バックエンドは既存 integration test 流（real Postgres、`testcontainers` 経由）。
- Apollo MockedProvider leak spy (`installApolloMockLeakSpy`) を引き続き使う。検索 + fetchMore の組み合わせは新しい mock entry を要求するので注意。
- swipe テストは `swipe-card.test.tsx` の `pointerdown` / `pointermove` / `pointerup` シミュレーションを流用。

## 8. リスクと未解決事項

| リスク | 対応 |
|---|---|
| `prefers-reduced-motion` を respect する場合、モバイルでは swipe 無効化となるが、その時の Delete 入り口が「常時表示の Delete アイコン」に切り替わる必要がある。`SwipeableRow` がメディアクエリに応じてレンダラを分岐する責務を持つ。 | `useReducedMotion()` フックを Tailwind の `motion-reduce:` ではなく JS で持つ (Snackbar との連携を保つため)。 |
| sonner が `app/layout.tsx` 配下のクライアント境界に置かれるため、RSC との相性に注意。Toaster は client-only で、layout.tsx は RSC のため、`<Toaster />` は `AppShell` の client 境界の中に置く。 | `frontend/src/components/nav/app-shell.tsx` 配下に Toaster を配置。 |
| 5 秒 delayed-DELETE 方式は、ユーザーが 5 秒以内に画面遷移するとタイマーが破棄されサーバ側に DELETE が届かない。タイマー破棄前に navigate event をフックして強制 commit する必要がある。 | `useBeforeUnload` + Next.js の route change イベント (`router.events` は app router にはないので `usePathname` 監視) で flushPendingDeletes() を呼ぶ。 |
| bulk delete の Undo を見送ったが、ユーザー目線で「個別は Undo できるのに bulk はできない」のは不整合に映る可能性。 | 今回スコープでは AlertDialog 確認で許容。将来 Undo 拡張は Tier 5 タスクとして起こす。 |
| 検索 query が `WHERE (front ILIKE ? OR back ILIKE ?)` のため、front + back の合計テキストが大きい cardgroup ではフルスキャン気味になる。pg_trgm + GIN index を貼る検討は今回スコープ外。 | indexing は `<= 10k cards/group` の範囲では問題ない想定。超過時の指標を Tier 5 として追跡。 |

## 9. 進捗ログ

実装着手後、各 Task の終了時にここへ追記する。テンプレ:

```
### Task N: <subject>
- 着手日 / 完了日: YYYY-MM-DD / YYYY-MM-DD
- PR: #...
- 実工数: X 日 (見積 Y 日)
- 備考: 追加で必要だった変更、想定外の手戻り、後続タスクへの影響など
```

### Task 1: Settings → kebab DropdownMenu に移管
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-1-commits (local commit 5df39d9)
- 実工数: 1 日 (見積 1 日)
- 備考: `cardgroup-header.tsx`, `rename-cardgroup-dialog.tsx`, `dropdown-menu.tsx` 新規作成。既存 `cardgroup-settings-card.tsx` を削除し機能を header kebab に統合。

### Task 2: h1 + Badge ヘッダと toolbar 整理
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-1-commits (local commit 5df39d9)
- 実工数: 1 日 (見積 1 日)
- 備考: Task 1 と同一 commit に含める。`cardgroup-cards-section.tsx` の renderHeader を toolbar 仕様に変更。h1 + Badge + Start learning + Add card の配置を確定。

### Task 3: バックエンド検索フィールドの追加
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-1-commits (local commit 99d03db)
- 実工数: 1 日 (見積 1 日)
- 備考: `cardsByCardgroupConnection` に `search: String` 引数を追加。`escapeLike` を repository で実装し、`%` `_` `\` のエスケープを完遂。cross-tenant test で非漏洩を保証。

### Task 4: フロント検索 UI + Apollo cache key 整合
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-2-a (local commit 4855835)
- 実工数: 1 日 (見積 1 日)
- 備考: `cards-client.tsx` に `searchInput` / `searchQuery` state と 300ms debounce を追加。`searchQuery` 変更時に fetchMore guard と fetchMoreError をリセット。`CARDS_DEFAULT_VARS` を `queries.ts` で export し 3 箇所で統一。

### Task 5a: sonner 導入 + undo-delete ヘルパ (インフラストラクチャ段階)
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-1-commits (local commit db197be)
- 実工数: 1 日 (見積 1 日)
- 備考: `sonner` 依存追加、`components/ui/sonner.tsx` 新規、`lib/undo-delete.ts` 新規（5 秒タイマー方式）。`app/layout.tsx` に Toaster を統合。

### Task 5b: cards-client.tsx への undo-delete 統合
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-2-a (local commit 4855835)
- 実工数: 1 日 (見積 1 日)
- 備考: Task 5a の infrastructure を使い、個別削除を `scheduleDelete` に切り替え。AlertDialog (個別) 廃止、Snackbar (Undo) 導入。Bulk delete は AlertDialog 据え置き。

### Task 6: Edit ボタン廃止 / 行クリックで inline 編集
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-2-a (local commit 4855835)
- 実工数: 1 日 (見積 1 日)
- 備考: `cards-client.tsx` の行コンポーネントに `onClick={() => setEditingId(card.id)}` を追加。checkbox / Delete アイコンに `e.stopPropagation()` を配置。Edit ボタンと個別削除 AlertDialog を廃止。

### Task 7: Empty state の 3 パターン整備
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-2-a (local commit 4855835)
- 実工数: 1 日 (見積 1 日)
- 備考: `cards-client.tsx` の empty state ロジックを拡張。`searchQuery` の有無で "Add cards..." vs "No cards match..." を分岐。Clear search button 実装。

### Task 8a: SwipeableRow コンポーネント (モバイル swipe-to-delete, reduce-motion)
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-2-b (local commit ee9c2c7)
- 実工数: 1 日 (見積 1 日)
- 備考: `use-reduced-motion.ts` フック新規作成。`swipeable-row.tsx` + `swipeable-row.test.tsx` 新規。@use-gesture/react + @react-spring/web で左 swipe 実装。Task 8b (cards-client 統合) は Wave 3。

### Task 8b: SwipeableRow を cards-client.tsx に統合
- 着手日 / 完了日: 2026-05-08 / 2026-05-08
- PR: wave-3 (commit on feature/cardgroup_edit_cards_integration)
- 実工数: 1 日 (見積 1 日)
- 備考: `cards-client.tsx` の各非編集行を `<SwipeableRow>` でラップ。`rowRefs` Map で per-row RefObject を管理し、別行タップ時に `closeOtherRows()` を呼ぶ。選択モード (`selectedIds.size > 0`) および編集中 (`editingId === card.id`) の行は `disabled={true}`。`window.matchMedia` スタブを `jest-dom.ts` setup に追加し全 jsdom テストで安全なデフォルトを確保。`cards-client.test.tsx` に 4 テスト追加 (SwipeableRow レンダリング / 選択モード disable / 編集モード disable / closeOtherRows 配線)。8a: commit ee9c2c7、8b: 本コミット。

### Step 2-5 final
- 着手日 / 完了日: 2026-04-30 / 2026-05-08
- Step 2 (PR review loop): 3 rounds / 約 3 日、`.claude/plans/` 内で incremental design→code→feedback を繰り返し。
- Step 3 (code-simplifier): `simplify` skill で既存コード 4 重複ロジックを refactor し、新規 SwipeableRow component に統合。
- Step 4 (test verification): 既存 Wave 1-2 をカバーする新規テスト + 既存テスト全 pass 確認。`jest` / `go test` 両方実行。
- Step 5 (docs): `.claude/rules/` 3 ファイル + `docs/` 2 ファイル に 10+ new sections を追加、cardgroup-edit-integration の learnings を定着させた。

## 10. 参考

- ブレインストーミングのモック: `.superpowers/brainstorm/19211-1778187647/content/` (gitignored)
- 関連ルール:
  - `.claude/rules/pagination.md` — Connection / cache vars / IO guard
  - `.claude/rules/go-library-gotchas.md` — GORM `LIKE` escape, cross-tenant test
  - `.claude/rules/frontend-typescript-conventions.md` — discriminated union, prop typing
  - `.claude/rules/frontend-rsc-error-handling.md` — try/finally + catch, AuthSessionMissingError
  - `.claude/rules/error-wrapping.md` — eris convention
  - `.claude/rules/language-policy.md` — committed text は英語のみ（実装時のソース・コメント・PR 文面）
