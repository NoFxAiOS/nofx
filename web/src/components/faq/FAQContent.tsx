import { useEffect, useRef } from 'react'
import { t, type Language } from '../../i18n/translations'
import type { FAQCategory } from '../../data/faqData'
// RoadmapWidget 移除动态嵌入，按需仅展示外部链接

const GITHUB_TASK_GUIDE: Record<
  Language,
  {
    linksLabel: string
    roadmapLabel: string
    taskBoardLabel: string
    steps: JSX.Element[]
    note: JSX.Element
  }
> = {
  zh: {
    linksLabel: '链接：',
    roadmapLabel: '路线图',
    taskBoardLabel: '任务看板',
    steps: [
      <>打开以上链接，按标签筛选（good first issue / help wanted / frontend / backend）。</>,
      <>打开任务，阅读描述与验收标准（Acceptance Criteria）。</>,
      <>评论“assign me”或自助分配（若权限允许）。</>,
      <>Fork 仓库到你的 GitHub 账户。</>,
      <>
        同步你的 fork 的 <code>dev</code> 分支与上游保持一致：
        <code className="ml-2">git remote add upstream https://github.com/NoFxAiOS/nofx.git</code>
        <br />
        <code>git fetch upstream</code>
        <br />
        <code>git checkout dev</code>
        <br />
        <code>git rebase upstream/dev</code>
        <br />
        <code>git push origin dev</code>
      </>,
      <>
        从你的 fork 的 <code>dev</code> 建立特性分支：
        <code className="ml-2">git checkout -b feat/your-topic</code>
      </>,
      <>
        推送到你的 fork：
        <code className="ml-2">git push origin feat/your-topic</code>
      </>,
      <>
        打开 PR：base 选择 <code>NoFxAiOS/nofx:dev</code> ← compare 选择 <code>你的用户名/nofx:feat/your-topic</code>。
      </>,
      <>
        在 PR 中关联 Issue（示例：<code className="ml-1">Closes #123</code>），选择正确 PR 模板；必要时与 <code>upstream/dev</code> 同步（rebase）后继续推送。
      </>,
    ],
    note: (
      <div
        className="rounded p-3 mt-3"
        style={{
          background: 'rgba(240, 185, 11, 0.08)',
          border: '1px solid rgba(240, 185, 11, 0.25)',
        }}
      >
        <div className="text-sm">
          <strong style={{ color: '#F0B90B' }}>提示：</strong> 参与贡献将享有激励制度（如
          Bounty/奖金、荣誉徽章与鸣谢、优先 Review/合并与内测资格 等）。 可在任务中优先选择带
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
      </div>
    ),
  },
  en: {
    linksLabel: 'Links:',
    roadmapLabel: 'Roadmap',
    taskBoardLabel: 'Task Dashboard',
    steps: [
      <>Open the links above and filter by labels (good first issue / help wanted / frontend / backend).</>,
      <>Open the task and read the Description & Acceptance Criteria.</>,
      <>Comment "assign me" or self-assign (if permitted).</>,
      <>Fork the repository to your GitHub account.</>,
      <>
        Sync your fork&apos;s <code>dev</code> with upstream:
        <code className="ml-2">git remote add upstream https://github.com/NoFxAiOS/nofx.git</code>
        <br />
        <code>git fetch upstream</code>
        <br />
        <code>git checkout dev</code>
        <br />
        <code>git rebase upstream/dev</code>
        <br />
        <code>git push origin dev</code>
      </>,
      <>
        Create a feature branch from your fork&apos;s <code>dev</code>:
        <code className="ml-2">git checkout -b feat/your-topic</code>
      </>,
      <>
        Push to your fork:
        <code className="ml-2">git push origin feat/your-topic</code>
      </>,
      <>
        Open a PR: base <code>NoFxAiOS/nofx:dev</code> ← compare <code>your-username/nofx:feat/your-topic</code>.
      </>,
      <>
        In PR, reference the Issue (e.g., <code className="ml-1">Closes #123</code>) and choose the proper PR template; rebase onto <code>upstream/dev</code> as needed.
      </>,
    ],
    note: (
      <div
        className="rounded p-3 mt-3"
        style={{
          background: 'rgba(240, 185, 11, 0.08)',
          border: '1px solid rgba(240, 185, 11, 0.25)',
        }}
      >
        <div className="text-sm">
          <strong style={{ color: '#F0B90B' }}>Note:</strong> Contribution incentives are available (e.g., cash bounties, badges & shout-outs, priority review/merge, beta access). Prefer tasks with
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
      </div>
    ),
  },
  es: {
    linksLabel: 'Enlaces:',
    roadmapLabel: 'Hoja de ruta',
    taskBoardLabel: 'Tablero de tareas',
    steps: [
      <>Abre los enlaces anteriores y filtra por etiquetas (good first issue / help wanted / frontend / backend).</>,
      <>Abre la tarea y lee la Descripción y los Criterios de Aceptación.</>,
      <>Comenta "assign me" o asígnatela tú mismo (si tienes permiso).</>,
      <>Haz fork del repositorio en tu cuenta de GitHub.</>,
      <>
        Sincroniza el <code>dev</code> de tu fork con upstream:
        <code className="ml-2">git remote add upstream https://github.com/NoFxAiOS/nofx.git</code>
        <br />
        <code>git fetch upstream</code>
        <br />
        <code>git checkout dev</code>
        <br />
        <code>git rebase upstream/dev</code>
        <br />
        <code>git push origin dev</code>
      </>,
      <>
        Crea una rama de características desde el <code>dev</code> de tu fork:
        <code className="ml-2">git checkout -b feat/tu-tema</code>
      </>,
      <>
        Haz push a tu fork:
        <code className="ml-2">git push origin feat/tu-tema</code>
      </>,
      <>
        Abre un PR: base <code>NoFxAiOS/nofx:dev</code> ← compare <code>tu-usuario/nofx:feat/tu-tema</code>.
      </>,
      <>
        En el PR, enlaza la Issue (por ejemplo, <code className="ml-1">Closes #123</code>) y elige la plantilla correcta; haz rebase sobre <code>upstream/dev</code> según sea necesario.
      </>,
    ],
    note: (
      <div
        className="rounded p-3 mt-3"
        style={{
          background: 'rgba(240, 185, 11, 0.08)',
          border: '1px solid rgba(240, 185, 11, 0.25)',
        }}
      >
        <div className="text-sm">
          <strong style={{ color: '#F0B90B' }}>Nota:</strong> Hay incentivos para contribuir (bounties/bonos, insignias y agradecimientos, revisión/fusión prioritaria, acceso beta). Prioriza las tareas con
          <a
            href="https://github.com/NoFxAiOS/nofx/labels/bounty"
            target="_blank"
            rel="noreferrer"
            style={{ color: '#F0B90B' }}
          >
            etiqueta bounty
          </a>
          , o tras completarla envía un
          <a
            href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
            target="_blank"
            rel="noreferrer"
            style={{ color: '#F0B90B' }}
          >
            Bounty Claim
          </a>
          .
        </div>
      </div>
    ),
  },
}

const PR_GUIDE_COPY: Record<
  Language,
  {
    referencesLabel: string
    steps: JSX.Element[]
    note: JSX.Element
  }
> = {
  zh: {
    referencesLabel: '参考文档：',
    steps: [
      <>
        Fork 仓库后，从你的 fork 的 <code>dev</code> 分支创建特性分支；避免直接向上游 <code>main</code> 提交。
      </>,
      <>分支命名：feat/…、fix/…、docs/…；提交信息遵循 Conventional Commits。</>,
      <>
        提交前运行检查：
        <code className="ml-2">
          npm --prefix web run lint && npm --prefix web
          run build
        </code>
      </>,
      <>涉及 UI 变更请附截图或短视频。</>,
      <>选择正确的 PR 模板（frontend/backend/docs/general）。</>,
      <>
        在 PR 中关联 Issue（示例：
        <code className="ml-1">Closes #123</code>），PR
        目标选择 <code>NoFxAiOS/nofx:dev</code>。
      </>,
      <>
        保持与 <code>upstream/dev</code> 同步（rebase），确保 CI 通过；尽量保持 PR 小而聚焦。
      </>,
    ],
    note: (
      <div className="rounded p-3 mt-3 bg-nofx-gold/10 border border-nofx-gold/25">
        <div className="text-sm">
          <strong className="text-nofx-gold">Note:</strong> 我们为高质量贡献提供激励（Bounty/奖金、荣誉徽章与鸣谢、优先 Review/合并与内测资格 等）。 详情可关注带
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
      </div>
    ),
  },
  en: {
    referencesLabel: 'References:',
    steps: [
      <>
        After forking, branch from your fork&apos;s <code>dev</code>; avoid direct commits to upstream <code>main</code>.
      </>,
      <>Branch naming: feat/…, fix/…, docs/…; commit messages follow Conventional Commits.</>,
      <>
        Run checks before PR:
        <code className="ml-2">
          npm --prefix web run lint && npm --prefix web
          run build
        </code>
      </>,
      <>For UI changes, attach screenshots or a short video.</>,
      <>Choose the proper PR template (frontend/backend/docs/general).</>,
      <>
        Link the Issue in PR (e.g., <code className="ml-1">Closes #123</code>) and target <code>NoFxAiOS/nofx:dev</code>.
      </>,
      <>Keep rebasing onto <code>upstream/dev</code>, ensure CI passes; prefer small and focused PRs.</>,
    ],
    note: (
      <div className="rounded p-3 mt-3 bg-nofx-gold/10 border border-nofx-gold/25">
        <div className="text-sm">
          <strong style={{ color: '#F0B90B' }}>Note:</strong> We offer contribution incentives (bounties, badges, shout-outs, priority review/merge, beta access). Look for tasks with
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
          after completion.
        </div>
      </div>
    ),
  },
  es: {
    referencesLabel: 'Referencias:',
    steps: [
      <>
        Tras hacer fork, crea la rama desde el <code>dev</code> de tu fork; evita commits directos al <code>main</code> upstream.
      </>,
      <>Nombra las ramas como feat/…, fix/…, docs/…; los mensajes de commit siguen Conventional Commits.</>,
      <>
        Ejecuta checks antes del PR:
        <code className="ml-2">
          npm --prefix web run lint && npm --prefix web
          run build
        </code>
      </>,
      <>Para cambios de UI, adjunta capturas o un video corto.</>,
      <>Elige la plantilla de PR adecuada (frontend/backend/docs/general).</>,
      <>
        Enlaza la Issue en el PR (ej., <code className="ml-1">Closes #123</code>) y apunta a <code>NoFxAiOS/nofx:dev</code>.
      </>,
      <>Haz rebase con frecuencia sobre <code>upstream/dev</code>, asegura que CI pase; prefiere PRs pequeños y enfocados.</>,
    ],
    note: (
      <div className="rounded p-3 mt-3 bg-nofx-gold/10 border border-nofx-gold/25">
        <div className="text-sm">
          <strong style={{ color: '#F0B90B' }}>Nota:</strong> Ofrecemos incentivos por contribución (bounties, insignias, agradecimientos, revisión/fusión prioritaria, acceso beta). Busca tareas con
          <a
            href="https://github.com/NoFxAiOS/nofx/labels/bounty"
            target="_blank"
            rel="noreferrer"
            style={{ color: '#F0B90B' }}
          >
            etiqueta bounty
          </a>
          , o envía un
          <a
            href="https://github.com/NoFxAiOS/nofx/blob/dev/.github/ISSUE_TEMPLATE/bounty_claim.md"
            target="_blank"
            rel="noreferrer"
            style={{ color: '#F0B90B' }}
          >
            Bounty Claim
          </a>
          tras completarla.
        </div>
      </div>
    ),
  },
}

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
        <div key={category.id} className="nofx-glass p-8 rounded-xl border border-white/5">
          {/* Category Header */}
          <div className="flex items-center gap-3 mb-6 pb-3 border-b border-white/10">
            <category.icon className="w-7 h-7 text-nofx-gold" />
            <h2 className="text-2xl font-bold text-nofx-text-main">
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
                <h3 className="text-xl font-semibold mb-3 text-nofx-text-main">
                  {t(item.questionKey, language)}
                </h3>

                {/* Answer */}
                <div className="prose prose-invert max-w-none text-nofx-text-muted leading-relaxed">
                  {item.id === 'github-projects-tasks' ? (
                    <div className="space-y-3">
                      {(() => {
                        const guide = GITHUB_TASK_GUIDE[language]
                        return (
                          <>
                            <div className="text-base">
                              {guide.linksLabel}{' '}
                              <a
                                href="https://github.com/orgs/NoFxAiOS/projects/3"
                                target="_blank"
                                rel="noreferrer"
                                style={{ color: '#F0B90B' }}
                              >
                                {guide.roadmapLabel}
                              </a>
                              {'  |  '}
                              <a
                                href="https://github.com/orgs/NoFxAiOS/projects/5"
                                target="_blank"
                                rel="noreferrer"
                                style={{ color: '#F0B90B' }}
                              >
                                {guide.taskBoardLabel}
                              </a>
                            </div>
                            <ol className="list-decimal pl-5 space-y-1 text-base">
                              {guide.steps.map((step, idx) => (
                                <li key={idx}>{step}</li>
                              ))}
                            </ol>
                            {guide.note}
                          </>
                        )
                      })()}
                    </div>
                  ) : item.id === 'contribute-pr-guidelines' ? (
                    <div className="space-y-3">
                      {(() => {
                        const guide = PR_GUIDE_COPY[language]
                        return (
                          <>
                            <div className="text-base">
                              {guide.referencesLabel}{' '}
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
                              {guide.steps.map((step, idx) => (
                                <li key={idx}>{step}</li>
                              ))}
                            </ol>
                            {guide.note}
                          </>
                        )
                      })()}
                    </div>
                  ) : (
                    <p className="text-base">{t(item.answerKey, language)}</p>
                  )}
                </div>

                {/* Divider */}
                <div className="mt-6 h-px bg-white/5" />
              </section>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
