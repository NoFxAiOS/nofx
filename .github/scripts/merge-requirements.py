#!/usr/bin/env python3
"""
依赖合并脚本 - 智能合并 requirements.txt
"""

import re
from pathlib import Path

def parse_requirements(content):
    """解析 requirements.txt 内容"""
    packages = {}
    
    for line in content.split('\n'):
        line = line.strip()
        
        # 跳过空行和注释
        if not line or line.startswith('#'):
            continue
        
        # 解析包名和版本
        if '==' in line:
            pkg, version = line.split('==', 1)
            packages[pkg.strip()] = version.strip()
        else:
            packages[line.strip()] = None
    
    return packages

def merge_requirements(our_packages, official_packages):
    """合并依赖包"""
    
    # 合并策略
    merged_packages = {}
    
    # 优先保留我们的关键包版本
    our_critical = ['streamlit', 'supabase', 'PyJWT']
    
    for pkg in our_critical:
        if pkg in our_packages:
            merged_packages[pkg] = our_packages[pkg]
            print(f"✅ 保留我们的: {pkg}=={our_packages[pkg]}")
    
    # 对于其他包，使用较新版本或官方版本
    all_packages = set(our_packages.keys()) | set(official_packages.keys())
    
    for pkg in all_packages:
        if pkg in merged_packages:
            continue
            
        if pkg in our_packages and pkg in official_packages:
            # 两个版本都存在，选择较新版本
            our_ver = our_packages[pkg]
            off_ver = official_packages[pkg]
            
            if our_ver and off_ver:
                # 简单的版本比较 (实际应该使用 packaging.version)
                if our_ver >= off_ver:
                    merged_packages[pkg] = our_ver
                    print(f"✅ 使用我们的较新版本: {pkg}=={our_ver}")
                else:
                    merged_packages[pkg] = off_ver
                    print(f"📥 使用官方的较新版本: {pkg}=={off_ver}")
            else:
                merged_packages[pkg] = our_packages[pkg] or official_packages[pkg]
        elif pkg in our_packages:
            merged_packages[pkg] = our_packages[pkg]
            print(f"✅ 保留我们的特有包: {pkg}")
        else:
            merged_packages[pkg] = official_packages[pkg]
            print(f"📥 添加官方特有包: {pkg}")
    
    return merged_packages

def generate_requirements_content(packages):
    """生成 requirements.txt 内容"""
    lines = [
        "# 自动合并的依赖文件",
        "# 🔄 集成官方依赖 + 我们的自定义依赖",
        ""
    ]
    
    # 添加包
    for pkg, version in sorted(packages.items()):
        if version:
            lines.append(f"{pkg}=={version}")
        else:
            lines.append(pkg)
    
    return '\n'.join(lines)

def main():
    # 读取我们的 requirements.txt
    our_path = "requirements.txt"
    if Path(our_path).exists():
        with open(our_path, 'r', encoding='utf-8') as f:
            our_content = f.read()
    else:
        print("❌ 找不到我们的 requirements.txt")
        return
    
    # 读取官方 requirements.txt
    official_path = "official_requirements.txt"
    if Path(official_path).exists():
        with open(official_path, 'r', encoding='utf-8') as f:
            official_content = f.read()
    else:
        print("❌ 找不到官方 requirements.txt")
        return
    
    print("🔧 开始合并依赖文件...")
    
    # 解析依赖
    our_packages = parse_requirements(our_content)
    official_packages = parse_requirements(official_content)
    
    print(f"📦 我们的包: {len(our_packages)} 个")
    print(f"📦 官方的包: {len(official_packages)} 个")
    
    # 合并依赖
    merged_packages = merge_requirements(our_packages, official_packages)
    
    # 生成合并后的内容
    merged_content = generate_requirements_content(merged_packages)
    
    # 保存结果
    with open(our_path, 'w', encoding='utf-8') as f:
        f.write(merged_content)
    
    print(f"✅ 依赖合并完成！总共 {len(merged_packages)} 个包")

if __name__ == "__main__":
    main()
