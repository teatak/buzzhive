#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-teatak/buzzhive}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
KIND="${1:-patch}"

if [ ! -f VERSION ]; then
  echo "❌ 错误: 根目录下未找到 VERSION 文件"
  exit 1
fi

CURRENT_VERSION=$(tr -d '[:space:]' < VERSION)
if [[ ! "$CURRENT_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "❌ 错误: VERSION 文件内容 ($CURRENT_VERSION) 不是合法的语义化版本 (格式应为 x.y.z)"
  exit 1
fi

# 1. 检查工作区是否存在除 VERSION 之外未提交的代码改动
DIRTY_FILES=$(git status --porcelain | grep -vE ' (VERSION|VERSION\.tmp)$' || true)
if [ -n "$DIRTY_FILES" ]; then
  echo "❌ 错误: 工作区存在未提交的代码改动，请先提交或 stash 后再执行发版:"
  echo "$DIRTY_FILES"
  exit 1
fi

# 静默尝试同步远端 tags，避免本地缺少最新 tag 判定失误
git fetch --tags origin >/dev/null 2>&1 || true

CURRENT_TAG="v$CURRENT_VERSION"

# 2. 幂等版本判定：如果当前版本的 tag 已经存在（本地或远端），才递增版本；否则继续发布当前版本
if git rev-parse -q --verify "refs/tags/$CURRENT_TAG" >/dev/null 2>&1; then
  # 当前版本已打过 tag，说明已发布过，需要递增版本
  IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"
  case "$KIND" in
    major)
      TARGET_VERSION="$((MAJOR + 1)).0.0"
      ;;
    minor)
      TARGET_VERSION="${MAJOR}.$((MINOR + 1)).0"
      ;;
    patch|*)
      TARGET_VERSION="${MAJOR}.${MINOR}.$((PATCH + 1))"
      ;;
  esac
  echo "📦 当前版本 $CURRENT_VERSION 已发布，递增版本: $CURRENT_VERSION -> $TARGET_VERSION ($KIND)"
  echo "$TARGET_VERSION" > VERSION
else
  # 当前版本未打 tag，说明上次未完成或刚就位，幂等复用当前版本，不重复递增
  TARGET_VERSION="$CURRENT_VERSION"
  echo "🔄 检测到当前版本 $TARGET_VERSION 尚未打 tag，继续发布该版本（不重复递增）..."
fi

TARGET_TAG="v$TARGET_VERSION"

# 3. 如果 VERSION 文件发生变动，先做 git commit，确保构建时带上干净的 commit
if ! git diff --quiet VERSION; then
  git add VERSION
  git commit -m "chore: bump version to $TARGET_VERSION"
  echo "✅ 已提交版本文件更新: chore: bump version to $TARGET_VERSION"
fi

# 4. 执行 Docker 构建与发布（若失败立即退出，此时不会创建和推送 tag，保证幂等）
echo "🐳 正在构建并推送 Docker 镜像 ($IMAGE:latest, $IMAGE:$TARGET_VERSION)..."
docker buildx build --platform "$PLATFORMS" -t "$IMAGE:latest" -t "$IMAGE:$TARGET_VERSION" --push .
echo "✅ Docker 镜像推送成功！"

# 5. Docker 成功后，创建本地 Git Tag 并推送到远程仓库
if ! git rev-parse -q --verify "refs/tags/$TARGET_TAG" >/dev/null 2>&1; then
  git tag -a "$TARGET_TAG" -m "Release $TARGET_TAG"
  echo "✅ 已创建本地 tag: $TARGET_TAG"
fi

echo "🚀 正在推送分支与标签到远程仓库..."
git push origin HEAD --follow-tags
echo "🎉 发布成功！版本: $TARGET_TAG"
