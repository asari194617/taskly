【Astro】i18n（国際化）機能で多言語対応のサイトを作る

Astro
JavaScript
2024/01/07
Astroを使用したサイトで多言語対応をしたので、やったことをまとめておく。
1. i18nルーティングの設定
今まで多言語対応するとなると、現在のパスからLocaleを判定し言語を出し分けするという風に自前で実装したりあとはコミュニティのライブラリを利用するかの2択だったが、Astro v4.0からi18nのルーティング機能が追加されたおかげでこの辺りの実装が非常に楽になった。
astro.config.mjs
import {defineConfig} from 'astro/config';

export default defineConfig({
  i18n: {
    defaultLocale: 'ja',
    locales: ['ja', 'en'],
  },
});
設定ファイルに対応させるlocalesを追記する。
astro.config.mjs
import {defineConfig} from 'astro/config';

export default defineConfig({
  i18n: {
    defaultLocale: 'ja',
    locales: ['ja', 'en'],
    routing: {
      prefixDefaultLocale: true,
    },
  },
});
筆者の場合は日本語（デフォルト言語）ページの場合でも/ja/xxxというパスにしたいので、prefixDefaultLocaleをtrueにしておく。
2. ヘルパーメソッドの実装
i18n.ts
export const LANGS = {
  ja: '日本語',
  en: 'English',
} as const;

type Lang = keyof typeof LANGS;
export type Multilingual = Record<Lang, string>;

export function useTranslations(lang: Lang) {
  return function t(multilingual: Multilingual): string {
    return multilingual[lang];
  };
}
多言語の文字列を扱うためのMultilingual型を定義
現在のlocaleに応じた言語の文字列を返すために、ヘルパーuseTranslationsを実装
const t = useTranslations('ja');
const msg = t({ ja: 'こんにちは', en: 'Hello' }); // => 'こんにちは'
useTranslationsにlocaleを渡すだけで翻訳用の関数を利用できる。
3. Astroファイル内で言語を出し分ける
---
const t = useTranslations(Astro.currentLocale as Lang);
---

<p>{t({ ja: 'こんにちは', en: 'Hello' })}</p>
Astroファイル内であればAstro.currentLocaleで現在のlocaleを取得できるので、これでコンポーネント内で出し分けできる。
4. 現在のlocaleに応じたパス・URLの生成
i18n.ts
export function getLocalePath(lang: Lang, path: string, hash?: string): string {
  const pathStr = path === '' ? '' : `/${path}`;
  return `${import.meta.env.BASE_URL}/${lang}${pathStr}${hash || ''}`;
}
筆者の場合、現在のlocaleに応じた相対パスを上記のように自前で生成しているが、
---
import { getRelativeLocaleUrl, getAbsoluteLocaleUrl } from 'astro:i18n';

getRelativeLocaleUrl('ja', 'xxx'); // => /ja/xxx
getAbsoluteLocaleUrl('ja', 'xxx'); // => https://example.com/ja/xxx
---
astro:i18nモジュールが提供するメソッドを使えば簡単に取得できる。
5. 現在のパス・URLを別Localeのものに変換
i18n.ts
type LocalePath = {
  path: string;
  lang: Lang;
  label: (typeof LANGS)[Lang];
};

export function generateLocalePaths(url: URL): LocalePath[] {
  const pathnames = url.pathname.split('/');

  return Object.keys(LANGS).map((lang) => {
    pathnames[2] = lang;
    return {
      path: pathnames.join('/').replace(/\/$/, ''),
      lang: lang as Lang,
      label: LANGS[lang as Lang],
    };
  });
}

export function generateLocaleUrls(url: URL): LocalePath[] {
  return generateLocalePaths(url).map((localePath) => ({
    ...localePath,
    path: baseOrigin + localePath.path,
  }));
}
後続の実装のために、現在のパス（URL）を別言語のものに変換できるようにしておく。
6. 言語切り替え機能の実装
lang-selector.astro
<select data-selector="lang-selector">
  {
    generateLocalePaths(Astro.url).map(({ path, lang, label }) => (
      <option label={label} value={path} selected={lang === Astro.currentLocale} />
    ))
  }
</select>

<script>
  function changeLang(): void {
    const select = document.querySelector('[data-selector="lang-selector"]');
    if (!select) return;

    select.addEventListener('change', ({ target: { value } }) =>
      location.href = value;
    );
  }
  changeLang();
</script>
セレクトボックスから言語を切り替えられるようにする。
<script>
  import { navigate } from 'astro:transitions/client';

  function changeLang(): void {
    const select = document.querySelector('[data-selector="lang-selector"]');
    if (!select) return;

    select.addEventListener('change', ({ target: { value } }) =>
      location.href = value;
      navigate(value),
    );
  }
  changeLang();
</script>
もしSPAモードにしている場合はastro:transitions/clientモジュールのnavigate()で遷移させる。
7. alternateタグの追加
<head>
  {/* i18n */}
  {
    generateLocaleUrls(currentUrl).map((props) => (
      <link rel="alternate" hreflang={props.lang} href={props.path} />
    ))
  }
</head>
サイトを国際化するにあたり各言語のページをindexさせたいので、alternateタグを追加する。