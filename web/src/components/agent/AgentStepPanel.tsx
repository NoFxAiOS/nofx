interface AgentStep {
  id: string
  label: string
  status: 'planning' | 'pending' | 'running' | 'completed' | 'replanned'
  detail?: string
}

interface AgentStepPanelProps {
  steps?: AgentStep[]
  visible?: boolean
}

const statusStyles: Record<AgentStep['status'], { dot: string; text: string }> = {
  planning: { dot: '#7c3aed', text: '#c4b5fd' },
  pending: { dot: 'rgba(255,255,255,0.18)', text: '#818198' },
  running: { dot: '#F0B90B', text: '#f6d67a' },
  completed: { dot: '#00e5a0', text: '#9cf5d5' },
  replanned: { dot: '#38bdf8', text: '#9bdcf7' },
}

export function AgentStepPanel({ steps, visible }: AgentStepPanelProps) {
  if (!visible || !steps || steps.length === 0) {
    return null
  }

  return (
    <div
      style={{
        marginBottom: 12,
        padding: '10px 12px',
        borderRadius: 12,
        background: 'linear-gradient(180deg, rgba(255,255,255,0.03), rgba(255,255,255,0.015))',
        border: '1px solid rgba(255,255,255,0.06)',
      }}
    >
      <div
        style={{
          fontSize: 11,
          fontWeight: 700,
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          color: '#7b7b91',
          marginBottom: 10,
        }}
      >
        Live Run
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {steps.map((step) => {
          const style = statusStyles[step.status]
          return (
            <div
              key={step.id}
              style={{
                display: 'grid',
                gridTemplateColumns: '14px 1fr',
                gap: 8,
                alignItems: 'start',
              }}
            >
              <span
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: 999,
                  marginTop: 5,
                  background: style.dot,
                  boxShadow:
                    step.status === 'running'
                      ? '0 0 0 4px rgba(240,185,11,0.08)'
                      : 'none',
                }}
              />
              <div>
                <div
                  style={{
                    fontSize: 12.5,
                    lineHeight: 1.5,
                    color: style.text,
                    fontWeight: step.status === 'running' ? 600 : 500,
                  }}
                >
                  {step.label}
                </div>
                {step.detail && (
                  <div
                    style={{
                      fontSize: 11.5,
                      lineHeight: 1.45,
                      color: '#6e6e86',
                      marginTop: 2,
                    }}
                  >
                    {step.detail}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
