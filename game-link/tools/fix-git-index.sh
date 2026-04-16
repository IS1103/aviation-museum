#!/bin/bash

# 修復 Git index 的腳本

cd "$(dirname "$0")/.."

echo "正在備份當前的 Git index..."
cp .git/index .git/index.backup

echo "正在重新構建 Git index..."
rm .git/index
git reset

echo "完成！Git index 已重新構建。"
echo "如果問題持續，可以恢復備份："
echo "  cp .git/index.backup .git/index"
