import { useEffect, useRef } from 'react'
import { t, type Language } from '../../i18n/translations'
import type { FAQCategory } from '../../data/faqData'
// RoadmapWidget: removed dynamic embedding; only show external links on demand

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
        <div key={category.id} className="nofx-glass p-8 rounded-xl border border-[rgba(26,24,19,0.14)]">
          {/* Category Header */}
          <div className="flex items-center gap-3 mb-6 pb-3 border-b border-[rgba(26,24,19,0.14)]">
            <category.icon className="w-7 h-7 text-nofx-gold" />
            <h2 className="text-2xl font-bold text-nofx-text">
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
                <h3 className="text-xl font-semibold mb-3 text-nofx-text">
                  {t(item.questionKey, language)}
                </h3>

                {/* Answer */}
                <div className="prose prose-invert max-w-none text-nofx-text-muted leading-relaxed">
                  {item.id === 'github-projects-tasks' ? (
                    <div className="space-y-3">
                      <div className="text-base">
                        {language === 'zh' ? 'Links:' : 'Links:'}{' '}
                        <a
                          href="https://github.com/orgs/NoFxAiOS/projects/3"
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: '#E0483B' }}
                        >
                          {language === 'zh' ? 'Roadmap' : 'Roadmap'}
                        </a>
                        {'  |  '}
                        <a
                          href="https://github.com/orgs/NoFxAiOS/projects/5"
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: '#E0483B' }}
                        >
                          {language === 'zh' ? 'Task Dashboard' : 'Task Dashboard'}
                        </a>
                      </div>
                      <ol className="list-decimal pl-5 space-y-1 text-base">
                        {language === 'zh' ? (
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
                          background: 'rgba(224, 72, 59, 0.08)',
                          border: '1px solid rgba(224, 72, 59, 0.25)',
                        }}
                      >
                        {language === 'zh' ? (
                          <div className="text-sm">
                            <strong style={{ color: '#E0483B' }}>Note:</strong>{' '}
                            Contribution incentives are available (e.g., cash
                            bounties, badges & shout-outs, priority
                            review/merge, beta access). Prefer tasks with
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
                            >
                              bounty label
                            </a>
                            , or file a
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
                            >
                              Bounty Claim
                            </a>
                            after completion.
                          </div>
                        ) : (
                          <div className="text-sm">
                            <strong style={{ color: '#E0483B' }}>Note:</strong>{' '}
                            Contribution incentives are available (e.g., cash
                            bounties, badges & shout-outs, priority
                            review/merge, beta access). Prefer tasks with
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
                            >
                              bounty label
                            </a>
                            , or file a
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
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
                        {language === 'zh' ? 'References:' : 'References:'}{' '}
                        <a
                          href="https://github.com/NoFxAiOS/nofx/blob/dev/CONTRIBUTING.md"
                          target="_blank"
                          rel="noreferrer"
                          className="text-nofx-gold hover:underline"
                        >
                          CONTRIBUTING.md
                        </a>
                        {'  |  '}
                        <a
                          href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/PR_TITLE_GUIDE.md"
                          target="_blank"
                          rel="noreferrer"
                          className="text-nofx-gold hover:underline"
                        >
                          PR_TITLE_GUIDE.md
                        </a>
                      </div>
                      <ol className="list-decimal pl-5 space-y-1 text-base">
                        {language === 'zh' ? (
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

                      <div className="rounded p-3 mt-3 bg-nofx-gold/10 border border-nofx-gold/25">
                        {language === 'zh' ? (
                          <div className="text-sm">
                            <strong className="text-nofx-gold">Note:</strong>{' '}
                            We offer contribution incentives (bounties, badges,
                            shout-outs, priority review/merge, beta access).
                            Look for tasks with
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
                            >
                              bounty label
                            </a>
                            , or submit a
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
                            >
                              Bounty Claim template
                            </a>
                            when ready.
                          </div>
                        ) : (
                          <div className="text-sm">
                            <strong style={{ color: '#E0483B' }}>Note:</strong>{' '}
                            We offer contribution incentives (bounties, badges,
                            shout-outs, priority review/merge, beta access).
                            Look for tasks with
                            <a
                              href="https://github.com/NoFxAiOS/nofx/labels/bounty"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
                            >
                              bounty label
                            </a>
                            , or submit a
                            <a
                              href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
                              target="_blank"
                              rel="noreferrer"
                              style={{ color: '#E0483B' }}
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
                <div className="mt-6 h-px bg-[rgba(26,24,19,0.14)]" />
              </section>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
