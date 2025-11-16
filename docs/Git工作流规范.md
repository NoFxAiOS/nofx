# Git 工作流规范

## 1. 总体目标

建立清晰、统一的 Git 工作流规范，解决团队协作中的冲突和代码丢失问题，同时有效管理开源和闭源两个版本的代码。

## 2. 分支管理策略

### 2.1 开源版本（GitHub Flow）

目前高频更新时期，针对开源版本的 GitHub Flow 工作流：

```
dev 分支
  ↓ Developer 新建功能分支
feat/hotfix 分支
  ↓ 开发、测试
  ↓ Pull Request
  ↓ Review
dev (合并回dev分支)
  ↓ 不定期完整测试
main (合并回main分支)  
  ↓ 自动发布
Release Tag
```

**分支规范：**
- `main`：唯一稳定分支
- `dev`：高频更新分支
- 功能开发分支：开发者自建分支，`feat/功能描述` 格式（从 `dev` 检出）
- 热修复分支：开发者自建分支，`hotfix/问题描述` 格式（从 `dev` 检出）

**操作流程：**
1. 从 `dev` 分支创建功能/修复分支
2. 在功能分支上进行开发
3. 开发完成后提交 Pull Request
4. 代码审查通过后合并到 `dev`
5. 不定期完整测试`dev`稳定后
6. 合并到`main`，自动触发 CI/CD 流程，发布 Release 并打 Tag

**完整测试频率建议：**
- 至少每周一次
- 重要功能发布后
- 紧急修复后

### 2.2 闭源版本（简化版 Git Flow）

针对闭源版本的 Git Flow 工作流：

```
test 分支(测试环境)
  ↓ Developer 新建功能分支
feat/hotfix 分支
  ↓ 开发、测试
  ↓ Pull Request
  ↓ Review
  ↓ 测试：测试人员拉取当前开发者的feature/hotfix分支，对feature/hotfix进行测试  
test-cp (合并进test-cp分支测试)
  ↓ 完整测试  --> 失败回滚
test (合并进test分支)
  ↓ 合并test分支  
  ↓ 更新test环境    
main (合并回main分支)  
  ↓ 自动发布
Release Tag
```


**分支规范：**
- `main`：生产环境稳定分支
- `test`：测试环境分支（从 `main` 检出）- 
- `test-cp`：测试者的临时测试环境分支（从 `test` 检出）
- 功能开发分支：开发者自建 `feat/功能描述` 格式（从 `test` 检出）
- 热修复分支：开发者自建 `hotfix/问题描述` 格式（从 `test` 检出）

**操作流程：**

**开发场景：**
```
1. test → 创建 feat/f-support-sql-driver
2. feat/f-support-sql-driver → 开发完成后PR
3. 测试人员拉取当前开发者的feature/hotfix分支合并到 test-cp，对test-cp进行测试
4. PR合并到 test
5. test测试验证通过 → 提交 PR 合并到 main
6. main → 完成自动触发 release 打 tag
```

## 3. 开源与闭源版本管理

### 3.1 仓库分离策略

```
上游 (开源版本)           下游 (闭源版本)
[公有仓库]              [私有仓库]
    |                       |
    |                       |
 开源核心代码 ←←←←←←←←←←←  商业版本完整代码
    |                       |
    |                       |
   社区贡献                 闭源功能
```

**仓库结构：**
- 公有仓库：存放开源版本代码，所有人可访问
- 私有仓库：存放商业版本完整代码（开源核心 + 闭源功能）

### 3.2 代码流向规范

**单向流动原则：**
- 开源核心的所有改进（新功能、Bug修复）定期从公有仓库合并到私有仓库
- 私有仓库中的闭源代码不流入公有仓库

### 3.3 实现方式

开源main分支发布后， 团队讨论后决定是否合并到闭源

## 4. 操作步骤详解

### 4.1 开源版本操作步骤

**新建功能开发：**
```bash
# 1. 切换到 dev 分支并更新
git checkout dev
git pull origin dev

# 2. 创建功能分支
git checkout -b feat/功能描述

# 3. 开发并提交代码
git add .
git commit -m "功能描述"

# 4. 推送分支到远程仓库
git push origin feat/功能描述

# 5. 在 GitHub 上创建 Pull Request
# 6. 代码审查通过后合并到dev
# 7. dev不定期完整测试后，并到main
```

### 4.2 闭源版本操作步骤

**新建功能开发：**
```bash
# 1. 切换到 test 分支并更新
git checkout test
git pull origin test

# 2. 创建功能分支
git checkout -b feat/功能描述

# 3. 开发并提交代码
git add .
git commit -m "功能描述"

# 4. 推送分支到远程仓库
git push origin feat/功能描述

# 5. 在 GitHub 上创建 Pull Request
# 6. 测试人员拉取当前开发者的feature/hotfix分支并到 test-cp，对test-cp进行测试
# 7. 测试人员test-cp分支完整测试验证通过
# 8. 测试通过后PR合并到 test分支，更新测试环境
# 9. 提交 PR 合并到 main
# 10. main → 完成自动触发 release 打 tag
```

## 5. 定期同步开源版本到闭源版本

为了确保闭源版本能够及时获得开源版本的改进，团队需要定期讨论是否将公有仓库的更新同步到私有仓库

**同步频率建议：**
- 每周一次定期同步
- 重要功能发布后立即同步
- 紧急修复后立即同步

## 6. GitHub里程碑自动Release案例

可以使用 GitHub Actions 来实现基于里程碑的自动发布，创建 `.github/workflows/auto-release.yml` 文件：

```yaml
name: Auto Release on Milestone Completion

on:
  milestone:
    types: [closed]
  workflow_dispatch:
    inputs:
      milestone_title:
        description: 'Milestone title to release'
        required: true

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v3
        
      - name: Get milestone info
        id: milestone
        run: |
          if [ "${{ github.event_name }}" = "milestone" ]; then
            MILESTONE_TITLE="${{ github.event.milestone.title }}"
            MILESTONE_DESC="${{ github.event.milestone.description }}"
          else
            MILESTONE_TITLE="${{ github.event.inputs.milestone_title }}"
            MILESTONE_DESC=""
          fi
          
          echo "milestone_title=$MILESTONE_TITLE" >> $GITHUB_OUTPUT
          echo "milestone_description=$MILESTONE_DESC" >> $GITHUB_OUTPUT
          
      - name: Create release tag
        run: |
          git config --global user.name 'github-actions[bot]'
          git config --global user.email 'github-actions[bot]@users.noreply.github.com'
          git tag -a "v${{ steps.milestone.outputs.milestone_title }}" -m "${{ steps.milestone.outputs.milestone_description }}"
          git push origin "v${{ steps.milestone.outputs.milestone_title }}"
          
      - name: Create GitHub Release
        uses: actions/create-release@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          tag_name: "v${{ steps.milestone.outputs.milestone_title }}"
          release_name: "Release ${{ steps.milestone.outputs.milestone_title }}"
          body: |
            ## Release Notes
            ${{ steps.milestone.outputs.milestone_description }}
            
            ### Features
            - Feature 1
            - Feature 2
            
            ### Bug Fixes
            - Bug fix 1
            - Bug fix 2
            
          draft: false
          prerelease: false
```

**使用说明：**
1. 当里程碑完成并关闭时，自动触发发布流程
2. 根据里程碑标题创建版本标签（如 v1.0.0）
3. 自动生成 GitHub Release 页面
4. 支持手动触发特定里程碑的发布

## 7. 冲突解决策略?

在将功能分支合并到 「test」/ 「dev」 分支时，可能会遇到代码冲突。为了解决这个问题并保持功能分支的独立性，推荐使用临时分支处理方式：

```bash
# 1. 创建临时分支用于解决冲突
git checkout -b test-tmp origin/test

# 2. 将功能分支合并到临时分支
git merge feat/功能分支名称

# 3. 解决冲突并提交
#    在编辑器中解决冲突文件
git add .
git commit -m "解决合并冲突"

# 4. 推送临时分支到远程仓库
git push origin test-tmp

# 5. 创建从 test-tmp 到 test 的 Pull Request
# 6. 代码审查通过后合并到 test 分支

# 7. 清理临时分支
git checkout test
git pull origin test
git branch -d test-tmp
git push origin --delete test-tmp
```

这种方式的优点：
- 保持功能分支的独立性，不会被 「test」 分支污染 「test」 分支可能包含其他正在测试的功能特性，避免相互影响
- 冲突解决过程在临时分支中进行，更加安全

## 8. 当前策略可能存在的问题
* 需要建立完整测试流程与feature/hotfix测试流程
* 需要专业测试人员，或者培训现有开发做测试
* 开源版本：dev完整测试的频率是否合适
* 同步开源版本到闭源版本的频率是否合适

## 9. 分支模型合并流转图

角色说明
- [开] 开发者（Developer）
- [评] 评审者（Reviewer）
- [维] 维护者（Maintainer）: 暂由Tester兼任
- [测] 测试人员或测试责任（可由 Maintainer/CI/QA 共同承担）
- [CI] CI/CD 系统（自动化构建/测试/发布）

### 9.1 开源版本分支模型
```text
Feature/Hotfix 分支 ───● 从Dev检出创建分支[开]──────────● 开发&自测[开]──────────● 提交PR→dev[开]                               
                                                                   
                                                                                             
Dev 分支                 ● 接收PR/Review[评]───● 合并到 dev[评]───● dev 不定期完整测试[测/CI][通过?]───● 发起 PR：dev→main[维]
                                                                  
                                                                   
Main 分支            ───● Review[评]───────────● 合并到 main[维]───────────● Release + Tag[CI]────────▶
```

### 9.2 闭源版本分支模型
```
Feature/Hotfix 分支 ───● 从Test检出创建分支[开]───────────● 开发&自测[开]──────────● 提交PR→test[开]      ● 修复并更新PR[开]────▶
                                                                                                   ╱
                                                                                                  ╱  测试失败回路：test 未通过 → 返回修复并更新 PR）
Test 分支         ● 接收PR/Review[评]───● 合并到test-cp[维]───● 构建环境[CI]───● 回归测试[测][通过?]───● 合并进test，发起PR：test→main[维]
                                                                                       
                                                                                     
Main 分支         ● Review[评]──────────● 合并到 main[维]──────────● Release + Tag[CI]────────▶

```
说明
- 失败回路：若 test-cp 分支的完整测试未通过，返回到 Feature/Hotfix 的“修复并更新PR”，开发者继续提交修复，PR 自动更新，再次进入 Review→合并→测试流程。


### 9.3 跨仓库代码流转图

```
┌─────────────────────────────────────────────────────────────┐
│                    跨仓库代码流转模型                        │
└─────────────────────────────────────────────────────────────┘

[公有仓库 - 开源版本]               [私有仓库 - 闭源版本]
       │                                 │
       ▼                                 ▼
    新功能开发                        商业功能开发
       │                                 │
       ▼                                 ▼
    代码审查                         闭源功能集成
       │                                 │
       ▼                                 │
    合并到main                            │
       │                                 │
       ▼                                 ▼
    发布Release                   定期同步开源更新
       │                                 │
       └─────────────────────────────────┘
                   │
                   ▼
              版本兼容性测试

```

## 10. 最佳实践

1. **提交信息规范：**
   - 使用清晰、简洁的提交信息
   - 遵循约定式提交格式：`<type>(<scope>): <subject>`

2. **分支命名规范：**
   - 功能分支：`feat/功能简述`
   - 热修复分支：`hotfix/问题简述`
   - 使用英文小写字母，单词间用连字符分隔

3. **代码审查：**
   - 所有合并到主分支的代码必须经过代码审查
   - 审查人员应关注代码质量、功能正确性和安全性

4. **定期同步：**
   - 定期将开源版本的改进同步到闭源版本