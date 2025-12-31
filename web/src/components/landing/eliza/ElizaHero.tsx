import { ArrowRight } from 'lucide-react'
import Prism from '../../Prism'

export default function ElizaHero() {
  return (
    <section
      className="relative w-full min-h-screen bg-nofx-bg overflow-hidden"
      style={{ scrollSnapAlign: 'start' }}
    >
      {/* Prism Background */}
      <div className="absolute inset-0 pointer-events-none">
        <Prism
          animationType="rotate"
          timeScale={0.3}
          scale={4}
          glow={1.2}
          noise={0.3}
          hueShift={0.5}
          colorFrequency={1.2}
          transparent={true}
          suspendWhenOffscreen={true}
        />
      </div>

      {/* Dark overlay for better text readability */}
      <div className="absolute inset-0 bg-nofx-bg/60 pointer-events-none" />

      {/* Mascot Image - Full right side */}
      <div className="absolute right-0 top-0 bottom-0 w-1/2 hidden lg:flex items-end justify-end pointer-events-none">
        {/* Glow effect behind mascot */}
        {/* <div className="absolute top-1/2 right-1/4 -translate-y-1/2 w-[60%] h-[60%] bg-nofx-gold/10 rounded-full blur-[120px]" /> */}
        <img
          src="/images/nofx_girl.png"
          alt="NOFX Mascot"
          className="relative z-10 h-[90vh] w-auto object-cover object-bottom"
          style={{
            filter: 'drop-shadow(0 0 80px rgba(240,185,11,0.3))',
          }}
        />
      </div>

      {/* Left Column - Text Content */}
      <div className="relative z-10 max-w-7xl mx-auto px-6 h-full">
        <div className="flex items-center min-h-screen py-20">
          <div className="flex flex-col justify-center max-w-xl">
            <h1 className="text-5xl md:text-6xl lg:text-7xl font-black leading-[1.1] tracking-tight mb-6">
              <span className="text-nofx-text">Your Agentic</span>
              <br />
              <span className="text-nofx-gold">Operating System</span>
            </h1>

            <p className="text-zinc-400 text-lg md:text-xl max-w-md mb-8 leading-relaxed">
              Deploy AI-powered trading agents across crypto, stocks, forex, and
              metals markets.
            </p>

            <div className="flex flex-wrap gap-4">
              <button
                onClick={() =>
                  document
                    .getElementById('welcome-section')
                    ?.scrollIntoView({ behavior: 'smooth' })
                }
                className="group px-8 py-4 bg-nofx-gold text-black font-bold rounded-sm hover:bg-white transition-all flex items-center gap-3"
              >
                <span>Get Started</span>
                <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Mobile Mascot - shown below text on small screens */}
      <div className="lg:hidden relative z-10 flex justify-center pb-10 -mt-10">
        <img
          src="/images/nofx_girl.png"
          alt="NOFX Mascot"
          className="h-[50vh] w-auto object-contain"
          style={{
            filter: 'drop-shadow(0 0 40px rgba(240,185,11,0.2))',
          }}
        />
      </div>
    </section>
  )
}
