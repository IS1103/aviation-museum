#!/bin/bash

# 手動提交腳本 - 用於解決 Cursor Git worker 問題

cd "$(dirname "$0")/.."

echo "正在檢查 Git 狀態..."
git status --short | wc -l | xargs echo "變更文件數："

echo ""
echo "正在添加所有變更..."
git add -A

echo ""
echo "變更摘要："
git status --short | head -20
echo "... (還有更多變更)"

echo ""
echo "請輸入提交訊息（或按 Enter 使用預設訊息）："
read -r commit_msg

if [ -z "$commit_msg" ]; then
    commit_msg="清理已刪除的客戶端文件"
fi

echo ""
echo "正在提交..."
git commit -m "$commit_msg"

echo ""
echo "提交完成！"
echo "當前分支：$(git branch --show-current)"
echo "提交哈希：$(git rev-parse HEAD)"
