#!/bin/bash

echo "🚀 GB28181/ONVIF视频监控平台 - 发布助手"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 检查GitHub CLI是否安装
if ! command -v gh &> /dev/null; then
    echo "❌ 未检测到GitHub CLI (gh)"
    echo ""
    echo "📝 手动发布步骤："
    echo ""
    echo "1️⃣  在GitHub上创建新仓库"
    echo "   访问: https://github.com/new"
    echo "   仓库名: gb28181-onvif-server"
    echo "   描述: GB28181/ONVIF视频监控平台，支持AI智能录像"
    echo ""
    echo "2️⃣  添加远程仓库并推送"
    echo "   git remote add origin git@github.com:YOUR_USERNAME/gb28181-onvif-server.git"
    echo "   或"
    echo "   git remote add origin https://github.com/YOUR_USERNAME/gb28181-onvif-server.git"
    echo ""
    echo "   git push -u origin master"
    echo "   git push --tags"
    echo ""
    echo "3️⃣  创建Release"
    echo "   访问: https://github.com/YOUR_USERNAME/gb28181-onvif-server/releases/new"
    echo "   - 选择标签: v1.0.0"
    echo "   - 标题: v1.0.0 - 完整的GB28181/ONVIF视频监控平台"
    echo "   - 描述: 粘贴 RELEASE_NOTES.md 的内容"
    echo "   - 发布Release"
    echo ""
    exit 1
fi

# 使用GitHub CLI创建仓库
echo "📦 正在创建GitHub仓库..."
gh repo create gb28181-onvif-server \
    --public \
    --description "GB28181/ONVIF视频监控平台，支持AI智能录像、流媒体服务、录像回放" \
    --source=. \
    --remote=origin

if [ $? -ne 0 ]; then
    echo "❌ 创建仓库失败，请手动创建"
    exit 1
fi

echo ""
echo "✅ 仓库创建成功"
echo ""

# 推送代码
echo "📤 正在推送代码到GitHub..."
git push -u origin master
git push --tags

echo ""
echo "✅ 代码推送成功"
echo ""

# 创建Release
echo "🎉 正在创建Release v1.0.0..."
gh release create v1.0.0 \
    --title "v1.0.0 - 完整的GB28181/ONVIF视频监控平台" \
    --notes-file RELEASE_NOTES.md

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎊 发布完成！"
echo ""
echo "📍 仓库地址: $(git remote get-url origin)"
echo "🏷️  版本标签: v1.0.0"
echo ""
echo "🌐 在线访问:"
echo "   - 仓库: https://github.com/$(gh repo view --json owner,name -q '.owner.login + "/" + .name')"
echo "   - Release: https://github.com/$(gh repo view --json owner,name -q '.owner.login + "/" + .name')/releases/tag/v1.0.0"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
