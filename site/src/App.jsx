import React from 'react';

    const { useState } = React;

    const navItems = ['使い方', '英単語整理', '機能', '料金'];
    const navHrefs = ['#review-flow', '#card-groups', '#features', '#pricing'];

    const logos = [
      'IELTS', 'TOEFL', 'Duolingo', 'Anki', '教室', 'Notion', 'Quizlet', 'Memrise', 'Teams'
    ];

    const featureSections = [
      {
        title: <>苦手な単語を、自然に復習へ。</>,
        description:
          '覚えたか、まだ不安かを短い操作で記録。弱点だけが残るので、復習の迷いが減ります。',
        visual: 'organized',
      },
      {
        title: <>単語リストを、すぐ学習カードに。</>,
        description:
          'CSVで取り込んだ単語を、目的別のカードグループとして整理。準備に時間をかけず、すぐ復習へ進めます。',
        visual: 'drafts',
      }
    ];

    const bento = [
      {
        icon: 'brand',
        title: 'スワイプで、迷う単語に集中',
        text: '左右スワイプで「覚えた／まだ不安」を判定。迷った単語を自動で優先表示し、開くたびに弱点だけを効率よく復習できます。',
      },
      {
        icon: '📥',
        title: '自分の英単語を、取り込んで整理',
        text: 'CSVで取り込んだ単語を、目的別のカードグループとして整理。準備に時間をかけず、すぐ復習へ進めます。',
      },
      {
        icon: '📊',
        title: '積み上げが見える',
        text: '学習済み・復習中・習得済みを見える化。毎日の積み上げと、弱点が減っていく様子が一目でわかります。',
      }
    ];

    function Logo() {
      return (
        <a href="#" className="flex items-center gap-[var(--fib-3)]" aria-label="Flamingo Armond home">
          <div className="brand-icon-shell flex h-[34px] w-[34px] items-center justify-center rounded-[10px] p-[5px]">
            <img className="brand-icon-img" src="data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDI0IiBoZWlnaHQ9IjEwMjQiIHZpZXdCb3g9IjAgMCAxMDI0IDEwMjQiPgogIDxyZWN0IHg9IjAiIHk9IjAiIHdpZHRoPSIxMDI0IiBoZWlnaHQ9IjEwMjQiIHJ4PSIxNTMiIHJ5PSIxNTMiIGZpbGw9IiNGRjZGNzkiLz4KICA8cGF0aCBkPSJNMCwwIEw5NSwwIEwxMDMsNiBMMTIyLDI1IEwxMjIsMjcgTDEyNCwyNyBMMTMxLDM1IEwxMzgsNDEgTDE0NSw0OSBMMTU2LDYwIEwxNTgsNjQgTDE1OCwxNDAgTDE1NCwxNDYgTDExOCwxODIgTDExMSwxOTAgTDMzLDI2OCBMMzEsMjY4IEwzMCwyNzEgTDI5LDMwMyBMMjc3LDMwMiBMMjg4LDI4NiBMMzAyLDI2NyBMMzA5LDI1NyBMMzIzLDIzOCBMMzM1LDIyMSBMMzQ4LDIwMyBMMzYyLDE4NCBMMzc0LDE2NyBMMzg3LDE0OSBMMzkzLDE0MSBMMzk4LDEzOCBMNDA1LDEzOCBMNDExLDE0MiBMNDEzLDE0NiBMNDEzLDE1MyBMNDA3LDE2MyBMMzk3LDE3NiBMMzg4LDE4OSBMMzc1LDIwNyBMMzYzLDIyNCBMMzQ5LDI0MyBMMzM2LDI2MSBMMzIyLDI4MSBMMzA4LDMwMCBMMzA3LDMwMiBMNDA2LDMwMyBMNDEyLDMwNyBMNDE0LDMxMSBMNDE0LDMxOSBMNDA3LDMyNyBMMzkwLDM0NCBMMzgyLDM1MSBMMzY1LDM2OCBMMzU3LDM3NSBMMzQyLDM5MCBMMzM0LDM5NyBMMzEzLDQxOCBMMzA1LDQyNSBMMjkyLDQzOCBMMjkwLDQzOCBMMjkwLDQ0MCBMMjgyLDQ0NyBMMjY1LDQ2NCBMMjU3LDQ3MSBMMjQwLDQ4OCBMMjMyLDQ5NSBMMjE0LDUxMyBMMjA2LDUyMCBMMTkzLDUzMyBMMTkyLDcxMyBMMjgzLDcxMyBMMjg5LDcxOCBMMjkwLDcyMCBMMjkwLDczMCBMMjg1LDczNiBMMjgyLDczNyBMNzgsNzM3IEw3Miw3MzMgTDcwLDczMCBMNzAsNzIwIEw3NSw3MTQgTDc4LDcxMyBMMTY4LDcxMyBMMTY3LDUzMyBMMTUxLDUxNyBMMTQzLDUxMCBMMTIwLDQ4NyBMMTEyLDQ4MCBMOTksNDY3IEw5MSw0NjAgTDcxLDQ0MCBMNjMsNDMzIEw0Nyw0MTcgTDQyLDQxMyBMMzcsNDA4IEwxOCwzODkgTDEwLDM4MiBMLTcsMzY1IEwtMTUsMzU4IEwtMjcsMzQ2IEwtMzUsMzM5IEwtNTMsMzIxIEwtNTQsMzE5IEwtNTQsMjQwIEwtNTEsMjM1IEwtMzEsMjE1IEwtMjYsMjEwIEwtMTYsMjAwIEwtOSwxOTIgTC00LDE4NyBMLTEsMTg2IEwtMSwxODQgTDEsMTg0IEwzLDE4MCBMOSwxNzUgTDE2LDE2NyBMMjMsMTYwIEwyOCwxNTUgTDY0LDExOSBMNjUsOTQgTDExLDk1IEwzLDEwMiBMLTQsMTEwIEwtMTIsMTE3IEwtMTQsMTIwIEwtMTUsMTY3IEwtMTgsMTcyIEwtMjEsMTc1IEwtMzAsMTc2IEwtMzcsMTcyIEwtMzksMTY4IEwtNDMsMTY2IEwtNzYsMTMzIEwtNzgsMTI5IEwtNzgsNzcgTC03NCw3MSBMLTY5LDY2IEwtNjcsNjYgTC02Nyw2NCBMLTY1LDY0IEwtNjUsNjIgTC02Myw2MiBMLTYxLDU4IEwtNTEsNDggTC00Myw0MSBMLTM2LDM0IEwtMjksMjYgTC0xNCwxMiBMLTMsMSBaICIgZmlsbD0iI0ZERkFGNiIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzQ0LDE1NSkiLz4KICA8cGF0aCBkPSJNMCwwIEwyNjgsMCBMMjY2LDUgTDI1NCwyMSBMMjQyLDM4IEwyMjksNTYgTDIxNiw3NCBMMjAzLDkyIEwxOTAsMTEwIEwxODAsMTI0IEwxNzksMTMyIEwxODIsMTM4IEwxODYsMTQwIEwxOTQsMTQwIEwxOTksMTM3IEwyMTAsMTIxIEwyMjMsMTAzIEwyMzYsODUgTDI0OSw2NyBMMjYzLDQ4IEwyNzcsMjggTDI5MSw5IEwyOTgsMCBMMzgxLDAgTDM3NSw3IEwzNjMsMTkgTDM1NSwyNiBMMzM4LDQzIEwzMzAsNTAgTDMxNCw2NiBMMzA2LDczIEwyODksOTAgTDI4MSw5NyBMMjYzLDExNSBMMjYxLDExNSBMMjU5LDExOSBMMjUxLDEyNiBMMjQwLDEzNyBMMjM0LDE0MiBMMjI5LDE0NyBMMjExLDE2NSBMMjAzLDE3MiBMMTkxLDE4NCBMMTg3LDE4MiBMMTcxLDE2NiBMMTYzLDE1OSBMMTQyLDEzOCBMMTM0LDEzMSBMMTE3LDExNCBMMTA5LDEwNyBMOTAsODggTDgyLDgxIEw2NSw2NCBMNTcsNTcgTDQxLDQxIEwzMywzNCBMMTYsMTcgTDgsMTAgTDAsMiBaICIgZmlsbD0iI0ZGNkY3OSIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzM0LDQ4MikiLz4KICA8cGF0aCBkPSJNMCwwIEw4MCwwIEw5MCwxMCBMOTUsMTUgTDExMiwzMiBMMTE5LDQwIEwxMjgsNDkgTDEyOCwxMDkgTDEwNywxMzAgTDEwMiwxMzUgTDkxLDE0NyBMODUsMTUyIEw4NCwxNTQgTDgyLDE1NCBMODIsMTU2IEw3NywxNjAgTDcyLDE2NiBMNzAsMTY2IEw2OCwxNzAgTDU2LDE4MiBMNTQsMTgyIEw1NCwxODQgTDUyLDE4NCBMNTIsMTg2IEw1MCwxODYgTDUwLDE4OCBMNDUsMTkyIEwzOCwyMDAgTDI2LDIxMiBMMTksMjE4IEwxMiwyMjYgTDcsMjMwIEw2LDIzMiBMNCwyMzIgTDIsMjM2IEwwLDIzOSBMLTEsMjgwIEwtMzgsMjgwIEwtMzgsMjI1IEwtMTIsMTk5IEwtNywxOTQgTDQsMTgzIEw5LDE3OCBMMjAsMTY3IEwyNywxNTkgTDc5LDEwNyBMODAsMTA0IEw4MCw1NiBMNzgsNTEgTDczLDQ5IEwtNSw0OSBMLTEyLDU0IEwtMTgsNjEgTC0yNiw2OCBMLTMxLDczIEwtMzgsODEgTC00Myw4NiBMLTQ0LDg4IEwtNDUsMTEzIEwtNTIsMTA3IEwtNjAsMTAwIEwtNjIsOTcgTC02Miw2MiBaICIgZmlsbD0iI0ZGNkY3OSIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzUxLDE3OCkiLz4KPC9zdmc+" alt="" />
          </div>
          <span className="hidden text-[20px] font-semibold tracking-[-0.015em] text-ink sm:block">
            Flamingo Armond
          </span>
          <span className="block text-[18px] font-semibold tracking-[-0.015em] text-ink sm:hidden">
            Flamingo
          </span>
        </a>
      );
    }

    function Button({ children, variant = 'primary', href = '#', className = '' }) {
      if (variant === 'primary') {
        return (
          <a
            href={href}
            className={`inline-flex min-h-[46px] items-center justify-center rounded-full bg-flamingo-500 px-[var(--button-x)] py-[var(--button-y)] text-[var(--button-text)] font-semibold text-white shadow-[0_2px_8px_rgba(255,95,79,0.18)] transition-colors hover:bg-flamingo-600 ${className}`}
          >
            {children}
          </a>
        );
      }

      return (
        <a
          href={href}
          className={`inline-flex min-h-[46px] items-center justify-center rounded-full border border-gray-300 bg-white px-[var(--button-x)] py-[var(--button-y)] text-[var(--button-text)] font-semibold text-gray-800 transition-colors hover:bg-gray-50 ${className}`}
        >
          {children}
        </a>
      );
    }

    function Header() {
      const [open, setOpen] = useState(false);

      return (
        <header className="page-shell relative z-50 flex h-[56px] items-center justify-between bg-white/95 backdrop-blur">
          <Logo />

          <nav className="hidden items-center gap-x-[var(--fib-1)] lg:flex">
            {navItems.map((item, index) => (
              <a
                key={item}
                href={navHrefs[index]}
                className="inline-flex h-9 items-center justify-center rounded-md px-[var(--fib-4)] py-2 text-[13px] font-medium text-gray-800 transition-colors hover:bg-gray-50"
              >
                {item}
                {index === 0 && <span className="ml-1 text-xs text-gray-400">⌄</span>}
              </a>
            ))}
          </nav>

          <div className="hidden items-center gap-[var(--fib-4)] md:flex">
            <Button variant="secondary" href="https://flamingo-armond-frontend.vercel.app/" className="!min-h-[36px] !px-[var(--fib-5)] !py-[var(--fib-2)] !text-sm">ログイン</Button>
            <Button href="https://flamingo-armond-frontend.vercel.app/" className="header-cta !text-sm">無料で始める</Button>
          </div>

          <button
            onClick={() => setOpen(!open)}
            className="flex h-10 w-10 items-center justify-center rounded-xl border border-gray-100 bg-white text-xl md:hidden"
            aria-label="Open menu"
          >
            ☰
          </button>

          {open && (
            <div className="mobile-menu-panel absolute left-4 right-4 top-16 z-50 rounded-3xl border border-gray-100 p-4 shadow-float md:hidden">
              <div className="grid gap-2">
                {navItems.map((item, index) => (
                  <a key={item} href={navHrefs[index]} onClick={() => setOpen(false)} className="rounded-2xl px-4 py-3 text-sm font-semibold text-gray-800 hover:bg-gray-50">
                    {item}
                  </a>
                ))}
                <div className="mt-3 flex gap-3">
                  <Button variant="secondary" href="https://flamingo-armond-frontend.vercel.app/" className="flex-1 !px-[var(--fib-5)] !py-[var(--fib-4)]">ログイン</Button>
                  <Button href="https://flamingo-armond-frontend.vercel.app/" className="flex-1 !px-[var(--fib-5)] !py-[var(--fib-4)]">始める</Button>
                </div>
              </div>
            </div>
          )}
        </header>
      );
    }

    function MailIcon() {
      return (
        <span className="brand-icon-shell inline-flex h-7 w-7 items-center justify-center rounded-lg p-1 shadow-sm">
          <img className="brand-icon-img" src="data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDI0IiBoZWlnaHQ9IjEwMjQiIHZpZXdCb3g9IjAgMCAxMDI0IDEwMjQiPgogIDxyZWN0IHg9IjAiIHk9IjAiIHdpZHRoPSIxMDI0IiBoZWlnaHQ9IjEwMjQiIHJ4PSIxNTMiIHJ5PSIxNTMiIGZpbGw9IiNGRjZGNzkiLz4KICA8cGF0aCBkPSJNMCwwIEw5NSwwIEwxMDMsNiBMMTIyLDI1IEwxMjIsMjcgTDEyNCwyNyBMMTMxLDM1IEwxMzgsNDEgTDE0NSw0OSBMMTU2LDYwIEwxNTgsNjQgTDE1OCwxNDAgTDE1NCwxNDYgTDExOCwxODIgTDExMSwxOTAgTDMzLDI2OCBMMzEsMjY4IEwzMCwyNzEgTDI5LDMwMyBMMjc3LDMwMiBMMjg4LDI4NiBMMzAyLDI2NyBMMzA5LDI1NyBMMzIzLDIzOCBMMzM1LDIyMSBMMzQ4LDIwMyBMMzYyLDE4NCBMMzc0LDE2NyBMMzg3LDE0OSBMMzkzLDE0MSBMMzk4LDEzOCBMNDA1LDEzOCBMNDExLDE0MiBMNDEzLDE0NiBMNDEzLDE1MyBMNDA3LDE2MyBMMzk3LDE3NiBMMzg4LDE4OSBMMzc1LDIwNyBMMzYzLDIyNCBMMzQ5LDI0MyBMMzM2LDI2MSBMMzIyLDI4MSBMMzA4LDMwMCBMMzA3LDMwMiBMNDA2LDMwMyBMNDEyLDMwNyBMNDE0LDMxMSBMNDE0LDMxOSBMNDA3LDMyNyBMMzkwLDM0NCBMMzgyLDM1MSBMMzY1LDM2OCBMMzU3LDM3NSBMMzQyLDM5MCBMMzM0LDM5NyBMMzEzLDQxOCBMMzA1LDQyNSBMMjkyLDQzOCBMMjkwLDQzOCBMMjkwLDQ0MCBMMjgyLDQ0NyBMMjY1LDQ2NCBMMjU3LDQ3MSBMMjQwLDQ4OCBMMjMyLDQ5NSBMMjE0LDUxMyBMMjA2LDUyMCBMMTkzLDUzMyBMMTkyLDcxMyBMMjgzLDcxMyBMMjg5LDcxOCBMMjkwLDcyMCBMMjkwLDczMCBMMjg1LDczNiBMMjgyLDczNyBMNzgsNzM3IEw3Miw3MzMgTDcwLDczMCBMNzAsNzIwIEw3NSw3MTQgTDc4LDcxMyBMMTY4LDcxMyBMMTY3LDUzMyBMMTUxLDUxNyBMMTQzLDUxMCBMMTIwLDQ4NyBMMTEyLDQ4MCBMOTksNDY3IEw5MSw0NjAgTDcxLDQ0MCBMNjMsNDMzIEw0Nyw0MTcgTDQyLDQxMyBMMzcsNDA4IEwxOCwzODkgTDEwLDM4MiBMLTcsMzY1IEwtMTUsMzU4IEwtMjcsMzQ2IEwtMzUsMzM5IEwtNTMsMzIxIEwtNTQsMzE5IEwtNTQsMjQwIEwtNTEsMjM1IEwtMzEsMjE1IEwtMjYsMjEwIEwtMTYsMjAwIEwtOSwxOTIgTC00LDE4NyBMLTEsMTg2IEwtMSwxODQgTDEsMTg0IEwzLDE4MCBMOSwxNzUgTDE2LDE2NyBMMjMsMTYwIEwyOCwxNTUgTDY0LDExOSBMNjUsOTQgTDExLDk1IEwzLDEwMiBMLTQsMTEwIEwtMTIsMTE3IEwtMTQsMTIwIEwtMTUsMTY3IEwtMTgsMTcyIEwtMjEsMTc1IEwtMzAsMTc2IEwtMzcsMTcyIEwtMzksMTY4IEwtNDMsMTY2IEwtNzYsMTMzIEwtNzgsMTI5IEwtNzgsNzcgTC03NCw3MSBMLTY5LDY2IEwtNjcsNjYgTC02Nyw2NCBMLTY1LDY0IEwtNjUsNjIgTC02Myw2MiBMLTYxLDU4IEwtNTEsNDggTC00Myw0MSBMLTM2LDM0IEwtMjksMjYgTC0xNCwxMiBMLTMsMSBaICIgZmlsbD0iI0ZERkFGNiIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzQ0LDE1NSkiLz4KICA8cGF0aCBkPSJNMCwwIEwyNjgsMCBMMjY2LDUgTDI1NCwyMSBMMjQyLDM4IEwyMjksNTYgTDIxNiw3NCBMMjAzLDkyIEwxOTAsMTEwIEwxODAsMTI0IEwxNzksMTMyIEwxODIsMTM4IEwxODYsMTQwIEwxOTQsMTQwIEwxOTksMTM3IEwyMTAsMTIxIEwyMjMsMTAzIEwyMzYsODUgTDI0OSw2NyBMMjYzLDQ4IEwyNzcsMjggTDI5MSw5IEwyOTgsMCBMMzgxLDAgTDM3NSw3IEwzNjMsMTkgTDM1NSwyNiBMMzM4LDQzIEwzMzAsNTAgTDMxNCw2NiBMMzA2LDczIEwyODksOTAgTDI4MSw5NyBMMjYzLDExNSBMMjYxLDExNSBMMjU5LDExOSBMMjUxLDEyNiBMMjQwLDEzNyBMMjM0LDE0MiBMMjI5LDE0NyBMMjExLDE2NSBMMjAzLDE3MiBMMTkxLDE4NCBMMTg3LDE4MiBMMTcxLDE2NiBMMTYzLDE1OSBMMTQyLDEzOCBMMTM0LDEzMSBMMTE3LDExNCBMMTA5LDEwNyBMOTAsODggTDgyLDgxIEw2NSw2NCBMNTcsNTcgTDQxLDQxIEwzMywzNCBMMTYsMTcgTDgsMTAgTDAsMiBaICIgZmlsbD0iI0ZGNkY3OSIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzM0LDQ4MikiLz4KICA8cGF0aCBkPSJNMCwwIEw4MCwwIEw5MCwxMCBMOTUsMTUgTDExMiwzMiBMMTE5LDQwIEwxMjgsNDkgTDEyOCwxMDkgTDEwNywxMzAgTDEwMiwxMzUgTDkxLDE0NyBMODUsMTUyIEw4NCwxNTQgTDgyLDE1NCBMODIsMTU2IEw3NywxNjAgTDcyLDE2NiBMNzAsMTY2IEw2OCwxNzAgTDU2LDE4MiBMNTQsMTgyIEw1NCwxODQgTDUyLDE4NCBMNTIsMTg2IEw1MCwxODYgTDUwLDE4OCBMNDUsMTkyIEwzOCwyMDAgTDI2LDIxMiBMMTksMjE4IEwxMiwyMjYgTDcsMjMwIEw2LDIzMiBMNCwyMzIgTDIsMjM2IEwwLDIzOSBMLTEsMjgwIEwtMzgsMjgwIEwtMzgsMjI1IEwtMTIsMTk5IEwtNywxOTQgTDQsMTgzIEw5LDE3OCBMMjAsMTY3IEwyNywxNTkgTDc5LDEwNyBMODAsMTA0IEw4MCw1NiBMNzgsNTEgTDczLDQ5IEwtNSw0OSBMLTEyLDU0IEwtMTgsNjEgTC0yNiw2OCBMLTMxLDczIEwtMzgsODEgTC00Myw4NiBMLTQ0LDg4IEwtNDUsMTEzIEwtNTIsMTA3IEwtNjAsMTAwIEwtNjIsOTcgTC02Miw2MiBaICIgZmlsbD0iI0ZGNkY3OSIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzUxLDE3OCkiLz4KPC9zdmc+" alt="" />
        </span>
      );
    }

    function HeroWords({ text }) {
      const words = text.includes('|') ? text.split('|') : text.split(' ');
      return (
        <>
          {words.map((word, index) => (
            <span
              key={`${word}-${index}`}
              className="hero-title-word animate-reveal"
              style={{ animationDelay: `${index * 75}ms`, opacity: 0 }}
            >
              {word === '続く' ? <em className="hero-title-em">{word}</em> : word}
            </span>
          ))}
        </>
      );
    }

    const carouselImages = [
      {
        src: `images/desktop-learn.png`,
        title: `PCでの学習画面`,
        description: `大きなカードで、単語と意味に集中できます。`,
        kind: `desktop`,
      },
      {
        src: `images/desktop-login.png`,
        title: `ログイン画面`,
        description: `Googleアカウントでスムーズに開始できます。`,
        kind: `desktop`,
      },
      {
        src: `images/mobile-manage.png`,
        title: `モバイル管理画面`,
        description: `スマホでもカードグループを確認できます。`,
        kind: `mobile`,
      },
      {
        src: `images/mobile-learn.png`,
        title: `モバイル学習画面`,
        description: `移動中でも片手で復習できます。`,
        kind: `mobile`,
      }
    ];

    function ProductPreview() {
      const [activeIndex, setActiveIndex] = React.useState(0);
      const active = carouselImages[activeIndex];

      React.useEffect(() => {
        const timer = window.setInterval(() => {
          setActiveIndex((current) => (current + 1) % carouselImages.length);
        }, 4200);

        return () => window.clearInterval(timer);
      }, []);

      return (
        <div className="relative w-full animate-reveal" style={{ opacity: 0, animationDelay: '575ms' }}>
          <div className="product-carousel-shell">
            <div className="product-carousel-stage">
              <button
                type="button"
                aria-label="前の画面"
                className="carousel-nav carousel-nav-left"
                onClick={() => setActiveIndex((activeIndex - 1 + carouselImages.length) % carouselImages.length)}
              >
                ‹
              </button>

              <div className="product-carousel-media" data-kind={active.kind}>
                <img
                  key={active.src}
                  src={active.src}
                  alt={active.title}
                  className="product-carousel-image"
                />
              </div>

              <button
                type="button"
                aria-label="次の画面"
                className="carousel-nav carousel-nav-right"
                onClick={() => setActiveIndex((activeIndex + 1) % carouselImages.length)}
              >
                ›
              </button>

              <div className="carousel-caption">
                <p className="carousel-title">{active.title}</p>
                <p className="carousel-description">{active.description}</p>
              </div>
            </div>

            <div className="carousel-dots" aria-label="画面プレビュー切り替え">
              {carouselImages.map((item, index) => (
                <button
                  key={item.title}
                  type="button"
                  aria-label={`${item.title}を表示`}
                  className={`carousel-dot ${index === activeIndex ? 'is-active' : ''}`}
                  onClick={() => setActiveIndex(index)}
                />
              ))}
            </div>
          </div>
        </div>
      );
    }


    function Hero() {
      return (
        <section className="hero-section mt-[var(--fib-7)] py-[var(--fib-7)] text-center md:mt-[var(--fib-8)] md:py-[var(--fib-7)]">
          <h1 className="helvetica-title hero-title-jp mx-auto max-w-[var(--hero-title-max)] text-[30px] text-ink sm:text-[42px] md:text-[54px] lg:text-[60px]">
            <HeroWords text="スワイプで続く|あなたの英単語学習コーチ" />
          </h1>

          <p className="hero-copy animate-reveal" style={{ opacity: 0, animationDelay: '450ms' }}>
            Flamingo Armondは、覚えたい単語をスワイプ式のカードに変えて、苦手な英単語を効率よく復習できる学習アプリです。<br />
            退屈な暗記ではなく、毎日少しずつ続けられる英単語学習を。
          </p>

          <div className="mt-[var(--fib-5)] md:mt-[var(--fib-6)]">
            <div className="hero-actions">
              <div className="flex animate-reveal items-center justify-center" style={{ opacity: 0, animationDelay: '500ms' }}>
                <Button href="https://flamingo-armond-frontend.vercel.app/" className="hero-main-cta">無料で始める</Button>
              </div>

            </div>

            <ProductPreview />
          </div>
        </section>
      );
    }

    function SectionVisual({ type }) {
      if (type === 'organized') {
        return (
          <div className="review-video-feature">
            <div className="review-video-copy">
              <p className="review-video-eyebrow">Review Flow</p>
              <h3>迷ったカードだけ、次の復習に残る。</h3>
              <p>
                右へ、左へ、迷ったら保留。<br />
                毎回の判断が学習履歴になり、次に見るべきカードを自動で整理します。
              </p>

              <div className="review-video-stats">
                <div>
                  <strong>Swipe</strong>
                  <span>知っているかを記録</span>
                </div>
                <div>
                  <strong>Review queue</strong>
                  <span>復習候補を自動更新</span>
                </div>
              </div>
            </div>

            <div className="review-video-device" aria-label="Flamingo Armondの学習デモ動画">
              <div className="review-video-phone">
                <div className="review-video-screen">
                  <video
                    src="videos/learn-demo.mp4"
                    className="review-video"
                    autoPlay
                    muted
                    loop
                    playsInline
                  />
                </div>
              </div>
            </div>
          </div>
        );
      }

      if (type === 'drafts') {
        return (
          <div className="review-video-feature cardgroup-video-feature is-reversed">
            <div className="review-video-copy">
              <p className="review-video-eyebrow">Card Groups</p>
              <h3>カードグループで、目的別に管理。</h3>
              <p>
                教材やスプレッドシートの単語をCSVで一括インポート。<br />
                タグやカテゴリで英検・TOEIC・受験用に整理し、開いた瞬間に復習できる状態をつくれます。
              </p>

              <div className="review-video-stats">
                <div>
                  <strong>Import</strong>
                  <span>CSVから一括追加</span>
                </div>
                <div>
                  <strong>Groups</strong>
                  <span>試験・教材ごとにカードグループを管理</span>
                </div>
              </div>
            </div>

            <div className="review-video-device" aria-label="カードグループ管理のデモ動画">
              <div className="review-video-phone">
                <div className="review-video-screen">
                  <video
                    src="videos/cardgroup-demo.mp4"
                    className="review-video"
                    autoPlay
                    muted
                    loop
                    playsInline
                  />
                </div>
              </div>
            </div>
          </div>
        );
      }

      return null;
    }

    function FeatureSection({ item }) {
      return (
        <section id={item.visual === "organized" ? "review-flow" : item.visual === "drafts" ? "card-groups" : undefined} className="copy-section">
          <h2 className="mx-auto helvetica-title text-[1.875rem] text-ink md:text-[2.5rem]">
            {item.title}
          </h2>
          <p className="section-copy">
            {item.description}
          </p>
          <div className="section-visual flex justify-center">
            <SectionVisual type={item.visual} />
          </div>
        </section>
      );
    }


    function Bento() {
      return (
        <section id="features" className="copy-section feature-principled-section">
          <h2 className="mx-auto helvetica-title text-[1.875rem] text-ink md:text-[2.5rem]">
            続けるために必要な機能を、ひとつに。
          </h2>
          <p className="section-copy">
            軽い操作感と、本格的な学習管理。<br />
            日本人の英語学習・資格対策・専門用語の暗記に使える設計です。
          </p>

          <div className="section-visual">
            <div className="feature-steps-panel">
              {bento.map((item, index) => (
                <div key={item.title} className="feature-step-card">
                  <div className="feature-step-card-copy">
                    <div className="feature-step-badge">
                      <span>{index + 1}</span>
                    </div>

                    <h3>{item.title}</h3>
                    <p>{item.text}</p>
                  </div>

                  <div className="feature-step-card-visual">
                    <div className="feature-step-icon">
                      {item.icon === 'brand' ? <img className="h-6 w-6" src="data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMDI0IiBoZWlnaHQ9IjEwMjQiIHZpZXdCb3g9IjAgMCAxMDI0IDEwMjQiPgogIDxyZWN0IHg9IjAiIHk9IjAiIHdpZHRoPSIxMDI0IiBoZWlnaHQ9IjEwMjQiIHJ4PSIxNTMiIHJ5PSIxNTMiIGZpbGw9IiNGRjZGNzkiLz4KICA8cGF0aCBkPSJNMCwwIEw5NSwwIEwxMDMsNiBMMTIyLDI1IEwxMjIsMjcgTDEyNCwyNyBMMTMxLDM1IEwxMzgsNDEgTDE0NSw0OSBMMTU2LDYwIEwxNTgsNjQgTDE1OCwxNDAgTDE1NCwxNDYgTDExOCwxODIgTDExMSwxOTAgTDMzLDI2OCBMMzEsMjY4IEwzMCwyNzEgTDI5LDMwMyBMMjc3LDMwMiBMMjg4LDI4NiBMMzAyLDI2NyBMMzA5LDI1NyBMMzIzLDIzOCBMMzM1LDIyMSBMMzQ4LDIwMyBMMzYyLDE4NCBMMzc0LDE2NyBMMzg3LDE0OSBMMzkzLDE0MSBMMzk4LDEzOCBMNDA1LDEzOCBMNDExLDE0MiBMNDEzLDE0NiBMNDEzLDE1MyBMNDA3LDE2MyBMMzk3LDE3NiBMMzg4LDE4OSBMMzc1LDIwNyBMMzYzLDIyNCBMMzQ5LDI0MyBMMzM2LDI2MSBMMzIyLDI4MSBMMzA4LDMwMCBMMzA3LDMwMiBMNDA2LDMwMyBMNDEyLDMwNyBMNDE0LDMxMSBMNDE0LDMxOSBMNDA3LDMyNyBMMzkwLDM0NCBMMzgyLDM1MSBMMzY1LDM2OCBMMzU3LDM3NSBMMzQyLDM5MCBMMzM0LDM5NyBMMzEzLDQxOCBMMzA1LDQyNSBMMjkyLDQzOCBMMjkwLDQzOCBMMjkwLDQ0MCBMMjgyLDQ0NyBMMjY1LDQ2NCBMMjU3LDQ3MSBMMjQwLDQ4OCBMMjMyLDQ5NSBMMjE0LDUxMyBMMjA2LDUyMCBMMTkzLDUzMyBMMTkyLDcxMyBMMjgzLDcxMyBMMjg5LDcxOCBMMjkwLDcyMCBMMjkwLDczMCBMMjg1LDczNiBMMjgyLDczNyBMNzgsNzM3IEw3Miw3MzMgTDcwLDczMCBMNzAsNzIwIEw3NSw3MTQgTDc4LDcxMyBMMTY4LDcxMyBMMTY3LDUzMyBMMTUxLDUxNyBMMTQzLDUxMCBMMTIwLDQ4NyBMMTEyLDQ4MCBMOTksNDY3IEw5MSw0NjAgTDcxLDQ0MCBMNjMsNDMzIEw0Nyw0MTcgTDQyLDQxMyBMMzcsNDA4IEwxOCwzODkgTDEwLDM4MiBMLTcsMzY1IEwtMTUsMzU4IEwtMjcsMzQ2IEwtMzUsMzM5IEwtNTMsMzIxIEwtNTQsMzE5IEwtNTQsMjQwIEwtNTEsMjM1IEwtMzEsMjE1IEwtMjYsMjEwIEwtMTYsMjAwIEwtOSwxOTIgTC00LDE4NyBMLTEsMTg2IEwtMSwxODQgTDEsMTg0IEwzLDE4MCBMOSwxNzUgTDE2LDE2NyBMMjMsMTYwIEwyOCwxNTUgTDY0LDExOSBMNjUsOTQgTDExLDk1IEwzLDEwMiBMLTQsMTEwIEwtMTIsMTE3IEwtMTQsMTIwIEwtMTUsMTY3IEwtMTgsMTcyIEwtMjEsMTc1IEwtMzAsMTc2IEwtMzcsMTcyIEwtMzksMTY4IEwtNDMsMTY2IEwtNzYsMTMzIEwtNzgsMTI5IEwtNzgsNzcgTC03NCw3MSBMLTY5LDY2IEwtNjcsNjYgTC02Nyw2NCBMLTY1LDY0IEwtNjUsNjIgTC02Myw2MiBMLTYxLDU4IEwtNTEsNDggTC00Myw0MSBMLTM2LDM0IEwtMjksMjYgTC0xNCwxMiBMLTMsMSBaICIgZmlsbD0iI0ZERkFGNiIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzQ0LDE1NSkiLz4KICA8cGF0aCBkPSJNMCwwIEwyNjgsMCBMMjY2LDUgTDI1NCwyMSBMMjQyLDM4IEwyMjksNTYgTDIxNiw3NCBMMjAzLDkyIEwxOTAsMTEwIEwxODAsMTI0IEwxNzksMTMyIEwxODIsMTM4IEwxODYsMTQwIEwxOTQsMTQwIEwxOTksMTM3IEwyMTAsMTIxIEwyMjMsMTAzIEwyMzYsODUgTDI0OSw2NyBMMjYzLDQ4IEwyNzcsMjggTDI5MSw5IEwyOTgsMCBMMzgxLDAgTDM3NSw3IEwzNjMsMTkgTDM1NSwyNiBMMzM4LDQzIEwzMzAsNTAgTDMxNCw2NiBMMzA2LDczIEwyODksOTAgTDI4MSw5NyBMMjYzLDExNSBMMjYxLDExNSBMMjU5LDExOSBMMjUxLDEyNiBMMjQwLDEzNyBMMjM0LDE0MiBMMjI5LDE0NyBMMjExLDE2NSBMMjAzLDE3MiBMMTkxLDE4NCBMMTg3LDE4MiBMMTcxLDE2NiBMMTYzLDE1OSBMMTQyLDEzOCBMMTM0LDEzMSBMMTE3LDExNCBMMTA5LDEwNyBMOTAsODggTDgyLDgxIEw2NSw2NCBMNTcsNTcgTDQxLDQxIEwzMywzNCBMMTYsMTcgTDgsMTAgTDAsMiBaICIgZmlsbD0iI0ZGNkY3OSIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzM0LDQ4MikiLz4KICA8cGF0aCBkPSJNMCwwIEw4MCwwIEw5MCwxMCBMOTUsMTUgTDExMiwzMiBMMTE5LDQwIEwxMjgsNDkgTDEyOCwxMDkgTDEwNywxMzAgTDEwMiwxMzUgTDkxLDE0NyBMODUsMTUyIEw4NCwxNTQgTDgyLDE1NCBMODIsMTU2IEw3NywxNjAgTDcyLDE2NiBMNzAsMTY2IEw2OCwxNzAgTDU2LDE4MiBMNTQsMTgyIEw1NCwxODQgTDUyLDE4NCBMNTIsMTg2IEw1MCwxODYgTDUwLDE4OCBMNDUsMTkyIEwzOCwyMDAgTDI2LDIxMiBMMTksMjE4IEwxMiwyMjYgTDcsMjMwIEw2LDIzMiBMNCwyMzIgTDIsMjM2IEwwLDIzOSBMLTEsMjgwIEwtMzgsMjgwIEwtMzgsMjI1IEwtMTIsMTk5IEwtNywxOTQgTDQsMTgzIEw5LDE3OCBMMjAsMTY3IEwyNywxNTkgTDc5LDEwNyBMODAsMTA0IEw4MCw1NiBMNzgsNTEgTDczLDQ5IEwtNSw0OSBMLTEyLDU0IEwtMTgsNjEgTC0yNiw2OCBMLTMxLDczIEwtMzgsODEgTC00Myw4NiBMLTQ0LDg4IEwtNDUsMTEzIEwtNTIsMTA3IEwtNjAsMTAwIEwtNjIsOTcgTC02Miw2MiBaICIgZmlsbD0iI0ZGNkY3OSIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMzUxLDE3OCkiLz4KPC9zdmc+" alt="" /> : item.icon}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>
      );
    }


    function Pricing() {
      return (
        <section id="pricing" className="copy-section">
          <h2 className="mx-auto helvetica-title text-[1.875rem] text-ink md:text-[2.5rem]">
            まずは無料ではじめられる。
          </h2>
          <p className="section-copy">
            個人学習から本格的な試験対策まで、必要に応じて使い方を広げられます。
          </p>

          <div className="section-visual mx-auto grid max-w-4xl gap-[var(--fib-6)] md:grid-cols-2">
            <div className="ratio-card p-[var(--fib-6)] text-left">
              <p className="text-sm font-bold uppercase tracking-[0.18em] text-flamingo-500">無料</p>
              <h3 className="mt-4 text-4xl font-black">¥0</h3>
              <p className="mt-[var(--fib-4)] text-sm leading-[1.72] text-[#6e6e73]">スワイプカード、英単語管理、基本的な復習を無料で始められます。</p>
              <Button href="https://flamingo-armond-frontend.vercel.app/" className="mt-8">始める</Button>
            </div>
            <div className="rounded-[var(--card-radius)] border border-flamingo-100 bg-gradient-to-b from-flamingo-50 to-white p-[var(--fib-6)] text-left shadow-float">
              <p className="text-sm font-bold uppercase tracking-[0.18em] text-flamingo-600">Pro</p>
              <h3 className="mt-4 text-4xl font-black">¥1,500<span className="text-base font-semibold text-[#6e6e73]"> / 月</span></h3>
              <p className="mt-[var(--fib-4)] text-sm leading-[1.72] text-[#6e6e73]">詳細な進捗分析、大量インポート、タグ管理、復習キューなどをより深く使えます。</p>
              <Button href="https://flamingo-armond-frontend.vercel.app/" className="mt-8">アップグレード</Button>
            </div>
          </div>
        </section>
      );
    }

    function CTA() {
      return (
        <section id="cta" className="copy-section final-cta-section">
          <div className="relative overflow-hidden rounded-[var(--panel-radius)] border border-[#E7E7E780] bg-gradient-to-b from-white to-flamingo-50 px-[var(--fib-5)] py-[var(--fib-7)] shadow-soft md:py-[var(--fib-8)]">
            <div className="absolute -left-24 -top-24 h-64 w-64 rounded-full bg-flamingo-100 blur-3xl"></div>
            <div className="absolute -bottom-28 -right-20 h-64 w-64 rounded-full bg-flamingo-100 blur-3xl"></div>

            <div className="relative mx-auto max-w-3xl">
              <h2 className="helvetica-title text-[2rem] text-ink md:text-[3.25rem]">
                続く英単語学習に。
              </h2>
              <p className="section-copy">
                最初の単語カードを追加して、短いセッションをスワイプ。<br />
                次に復習すべき単語は、Flamingo Armondがわかりやすく整理します。
              </p>
              <div className="mt-[var(--fib-6)] flex justify-center">
                <Button href="https://flamingo-armond-frontend.vercel.app/">無料で始める</Button>
              </div>
            </div>
          </div>
        </section>
      );
    }

    function Footer() {
      return (
        <footer className="border-t border-gray-100 pb-[var(--fib-8)] pt-[var(--fib-7)]">
          <div className="page-shell flex flex-col justify-between gap-[var(--fib-6)] md:flex-row">
            <div>
              <Logo />
              <p className="mt-4 max-w-sm text-sm leading-6 text-[#6e6e73]">
                スワイプで続ける英単語学習フラッシュカード。<br />
                日本人学習者に向けた、軽くて続けやすい学習体験を届けます。
              </p>
            </div>

            <div className="grid grid-cols-2 gap-[var(--fib-7)] text-sm md:gap-[var(--fib-8)]">
              {[
                ['プロダクト', ['機能', '料金']],
                ['リソース', ['概要', 'GitHub']],
              ].map(([title, links]) => (
                <div key={title}>
                  <p className="font-bold text-ink">{title}</p>
                  <div className="mt-[var(--fib-4)] grid gap-[var(--fib-4)] text-[#6e6e73]">
                    {links.map((link) => {
                      const isExternal = link === 'GitHub';
                      return (
                        <a
                          key={link}
                          href={isExternal ? 'https://github.com/yasuflatland-lf/flamingo-armond' : '#'}
                          className="hover:text-flamingo-600"
                          {...(isExternal ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
                        >
                          {link}
                        </a>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="page-shell mt-[var(--fib-6)] text-sm text-gray-400">
            © 2026 Flamingo Armond. All rights reserved.
          </div>
        </footer>
      );
    }

    function App() {
      return (
        <div className="pt-[var(--fib-5)]">
          <Header />
          <main className="page-shell isolate">
            <Hero />
            {featureSections.map((section) => <FeatureSection key={section.visual} item={section} />)}
            <Bento />
            <Pricing />
            <CTA />
          </main>
          <Footer />
        </div>
      );
    }

export default App;
