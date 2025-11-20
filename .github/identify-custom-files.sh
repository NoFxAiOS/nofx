#!/bin/bash
# 识别与官方仓库不同的文件（您的自定义文件）

echo "识别自定义文件..."

# 添加上游仓库
git remote add upstream https://github.com/NoFxAiOS/nofx.git
git fetch upstream

# 获取差异文件列表
git diff --name-only upstream/main HEAD > .github/changed-files.txt

echo "以下文件可能与官方版本不同："
cat .github/changed-files.txt

# 创建自定义文件列表（手动审查后使用）
echo "请手动审查 .github/changed-files.txt"
echo "然后将您的自定义文件添加到 .github/custom-files.txt"
