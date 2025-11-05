import { useEffect, useRef } from 'react'
import { t, type Language } from '../../i18n/translations'
import type { FAQCategory } from '../../data/faqData'
// RoadmapWidget 移除动态嵌入，按需仅展示外部链接

interface FAQContentProps {
  categories: FAQCategory[]
  language: Language
  onActiveItemChange: (itemId: string) => void
}

export function FAQContent({
  categories,
  language,
  onActiveItemChange,
}: FAQContentProps) {
  const sectionRefs = useRef<Map<string, HTMLElement>>(new Map())

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const itemId = entry.target.getAttribute('data-item-id')
            if (itemId) {
              onActiveItemChange(itemId)
            }
          }
        })
      },
      {
        rootMargin: '-100px 0px -80% 0px',
        threshold: 0,
      }
    )

    sectionRefs.current.forEach((ref) => {
      if (ref) observer.observe(ref)
    })

    return () => {
      sectionRefs.current.forEach((ref) => {
        if (ref) observer.unobserve(ref)
      })
    }
  }, [onActiveItemChange])

  const setRef = (itemId: string, element: HTMLElement | null) => {
    if (element) {
      sectionRefs.current.set(itemId, element)
    } else {
      sectionRefs.current.delete(itemId)
    }
  }

  return (
    <div className="space-y-12">
      {categories.map((category) => (
        <div key={category.id}>
          {/* Category Header */}
          <div
            className="flex items-center gap-3 mb-6 pb-3"
            style={{ borderBottom: '2px solid #2B3139' }}
          >
            <category.icon className="w-7 h-7" style={{ color: '#F0B90B' }} />
            <h2 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
              {t(category.titleKey, language)}
            </h2>
          </div>

          {/* FAQ Items */}
          <div className="space-y-8">
            {category.items.map((item) => (
              <section
                key={item.id}
                id={item.id}
                data-item-id={item.id}
                ref={(el) => setRef(item.id, el)}
                className="scroll-mt-24"
              >
                {/* Question */}
                <h3
                  className="text-xl font-semibold mb-3"
                  style={{ color: '#EAECEF' }}
                >
                  {t(item.questionKey, language)}
                </h3>

                {/* Answer */}
                <div
                  className="prose prose-invert max-w-none"
                  style={{
                    color: '#B7BDC6',
                    lineHeight: '1.7',
                  }}
                >
                  {item.id === 'github-projects-tasks' ? (
                    <div className="space-y-3">
                      <div className="text-base">
                        {language === 'zh' ? '链接：' : 'Links:'}{' '}
                        <a
                          href="https://github.com/orgs/NoFxAiOS/projects/3"
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: '#F0B90B' }}
                        >
                          {language === 'zh' ? '路线图' : 'Roadmap'}
                        </a>
                        {'  |  '}
                        <a
                          href="https://github.com/orgs/NoFxAiOS/projects/5"
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: '#F0B90B' }}
                        >
                          {language === 'zh' ? '任务看板' : 'Task Dashboard'}
                        </a>
                      </div>
                      <ol className="list-decimal pl-5 space-y-1 text-base">
                        {language === 'zh' ? (
                          <>
                            <li>
                              打开以上链接，按标签筛选（good first issue / help
                              wanted / frontend / backend）。
                            </li>
                            <li>
                              打开任务，阅读描述与验收标准（Acceptance
                              Criteria）。
                            </li>
                            <li>评论“assign me”或自助分配（若权限允许）。</li>
                            <li>Fork 仓库到你的 GitHub 账户。</li>
                            <li>
                              同步你的 fork 的 <code>dev</code>{' '}
                              分支与上游保持一致：
                              <code className="ml-2">
                                git remote add upstream
                                https://github.com/NoFxAiOS/nofx.git
                              </code>
                              <br />
                              <code>git fetch upstream</code>
                              <br />
                              <code>git checkout dev</code>
                              <br />
                              <code>git rebase upstream/dev</code>
                              <br />
                              <code>git push origin dev</code>
                            </li>
                            <li>
                              从你的 fork 的 <code>dev</code> 建立特性分支：
                              <code className="ml-2">
                                git checkout -b feat/your-topic
                              </code>
                            </li>
                            <li>
                              推送到你的 fork：
                              <code className="ml-2">
                                git push origin feat/your-topic
                              </code>
                            </li>
                            <li>
                              打开 PR：base 选择 <code>NoFxAiOS/nofx:dev</code>{' '}
                              ← compare 选择{' '}
                              <code>你的用户名/nofx:feat/your-topic</code>。
                            </li>
                            <li>
                              在 PR 中关联 Issue（示例：
                              <code className="ml-1">Closes #123</code>
                              ），选择正确 PR 模板；必要时与{' '}
                              <code>upstream/dev</code>{' '}
                              同步（rebase）后继续推送。
                            </li>
                          </>
                        ) : (
                          <>
                            <li>
                              Open the links above and filter by labels (good
                              first issue / help wanted / frontend / backend).
                            </li>
                            <li>
                              Open the task and read the Description &
                              Acceptance Criteria.
                            </li>
                            <li>
                              Comment "assign me" or self-assign (if permitted).
                            </li>
                            <li>Fork the repository to your GitHub account.</li>
                            <li>
                              Sync your fork's <code>dev</code> with upstream:
                              <code className="ml-2">
                                git remote add upstream
                                https://github.com/NoFxAiOS/nofx.git
                              </code>
                              <br />
                              <code>git fetch upstream</code>
                              <br />
                              <code>git checkout dev</code>
                              <br />
                              <code>git rebase upstream/dev</code>
                              <br />
                              <code>git push origin dev</code>
                            </li>
                            <li>
                              Create a feature branch from your fork's{' '}
                              <code>dev</code>:
                              <code className="ml-2">
                                git checkout -b feat/your-topic
                              </code>
                            </li>
                            <li>
                              Push to your fork:
                              <code className="ml-2">
                                git push origin feat/your-topic
                              </code>
                            </li>
                            <li>
                              Open a PR: base <code>NoFxAiOS/nofx:dev</code> ←
                              compare{' '}
                              <code>your-username/nofx:feat/your-topic</code>.
                            </li>
                            <li>
                              In PR, reference the Issue (e.g.,{' '}
                              <code className="ml-1">Closes #123</code>) and
                              choose the proper PR template; rebase onto{' '}
                              <code>upstream/dev</code> as needed.
                            </li>
                          </>
                        )}
                      </ol>

                      <div
                        className="rounded p-3 mt-3"
                        style={{
                          background: 'rgba(240, 185, 11, 0.08)',
                          border: '1px solid rgba(240, 185, 11, 0.25)',
                        }}
                      >
                        {language === 'zh' ? (
                          <div className="text-sm">
                            <strong style={{ color: '#F0B90B' }}>提示：</strong>{' '}
                            参与贡献将享有激励制度（如
                            Bounty/奖金、荣誉徽章与鸣谢、优先
                            Review/合并与内测资格 等）。 可在任务中优先选择带
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              bounty 标签
                            </a>
                            的事项，或完成后提交
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              Bounty Claim
                            </a>
                            申请。
                          </div>
                        ) : (
                          <div className="text-sm">
                            <strong style={{ color: '#F0B90B' }}>Note:</strong>{' '}
                            Contribution incentives are available (e.g., cash
                            bounties, badges & shout-outs, priority
                            review/merge, beta access). Prefer tasks with
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              bounty label
                            </a>
                            , or file a
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              Bounty Claim
                            </a>
                            after completion.
                          </div>
                        )}
                      </div>
                    </div>
                  ) : item.id === 'contribute-pr-guidelines' ? (
                    <div className="space-y-3">
                      <div className="text-base">
                        {language === 'zh' ? '参考文档：' : 'References:'}{' '}
                        <a
                          href="https://github.com/NoFxAiOS/nofx/blob/dev/CONTRIBUTING.md"
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: '#F0B90B' }}
                        >
                          CONTRIBUTING.md
                        </a>
                        {'  |  '}
                        <a
                          href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/PR_TITLE_GUIDE.md"
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: '#F0B90B' }}
                        >
                          PR_TITLE_GUIDE.md
                        </a>
                      </div>
                      <ol className="list-decimal pl-5 space-y-1 text-base">
                        {language === 'zh' ? (
                          <>
                            <li>
                              Fork 仓库后，从你的 fork 的 <code>dev</code>{' '}
                              分支创建特性分支；避免直接向上游 <code>main</code>{' '}
                              提交。
                            </li>
                            <li>
                              分支命名：feat/…、fix/…、docs/…；提交信息遵循
                              Conventional Commits。
                            </li>
                            <li>
                              提交前运行检查：
                              <code className="ml-2">
                                npm --prefix web run lint && npm --prefix web
                                run build
                              </code>
                            </li>
                            <li>涉及 UI 变更请附截图或短视频。</li>
                            <li>
                              选择正确的 PR
                              模板（frontend/backend/docs/general）。
                            </li>
                            <li>
                              在 PR 中关联 Issue（示例：
                              <code className="ml-1">Closes #123</code>），PR
                              目标选择 <code>NoFxAiOS/nofx:dev</code>。
                            </li>
                            <li>
                              保持与 <code>upstream/dev</code>{' '}
                              同步（rebase），确保 CI 通过；尽量保持 PR
                              小而聚焦。
                            </li>
                          </>
                        ) : (
                          <>
                            <li>
                              After forking, branch from your fork's{' '}
                              <code>dev</code>; avoid direct commits to upstream{' '}
                              <code>main</code>.
                            </li>
                            <li>
                              Branch naming: feat/…, fix/…, docs/…; commit
                              messages follow Conventional Commits.
                            </li>
                            <li>
                              Run checks before PR:
                              <code className="ml-2">
                                npm --prefix web run lint && npm --prefix web
                                run build
                              </code>
                            </li>
                            <li>
                              For UI changes, attach screenshots or a short
                              video.
                            </li>
                            <li>
                              Choose the proper PR template
                              (frontend/backend/docs/general).
                            </li>
                            <li>
                              Link the Issue in PR (e.g.,{' '}
                              <code className="ml-1">Closes #123</code>) and
                              target <code>NoFxAiOS/nofx:dev</code>.
                            </li>
                            <li>
                              Keep rebasing onto <code>upstream/dev</code>,
                              ensure CI passes; prefer small and focused PRs.
                            </li>
                          </>
                        )}
                      </ol>

                      <div
                        className="rounded p-3 mt-3"
                        style={{
                          background: 'rgba(240, 185, 11, 0.08)',
                          border: '1px solid rgba(240, 185, 11, 0.25)',
                        }}
                      >
                        {language === 'zh' ? (
                          <div className="text-sm">
                            <strong style={{ color: '#F0B90B' }}>提示：</strong>{' '}
                            我们为高质量贡献提供激励（Bounty/奖金、荣誉徽章与鸣谢、优先
                            Review/合并与内测资格 等）。 详情可关注带
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              bounty 标签
                            </a>
                            的任务，或使用
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              Bounty Claim 模板
                            </a>
                            提交申请。
                          </div>
                        ) : (
                          <div className="text-sm">
                            <strong style={{ color: '#F0B90B' }}>Note:</strong>{' '}
                            We offer contribution incentives (bounties, badges,
                            shout-outs, priority review/merge, beta access).
                            Look for tasks with
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              bounty label
                            </a>
                            , or submit a
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#F0B90B' }}
                            >
                              Bounty Claim
                            </a>
                            when ready.
                          </div>
                        )}
                      </div>
                    </div>
                  ) : (
                    <p className="text-base">{t(item.answerKey, language)}</p>
                  )}
                </div>

                {/* Divider */}
                <div className="mt-6 h-px" style={{ background: '#2B3139' }} />
              </section>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
