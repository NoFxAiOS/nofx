export default function AnimatedSection({ children }: { children: React.ReactNode }) {
  // 轻量容器：统一间距与可读性，避免引入额外依赖
  return <section className='py-14 md:py-20'>{children}</section>
}

