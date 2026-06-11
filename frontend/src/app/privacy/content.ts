type Section = { title: string; body: string };

type PrivacyContent = {
  title: string;
  lastUpdated: string;
  backToLogin: string;
  intro: string;
  sections: Section[];
};

export const PRIVACY_EN: PrivacyContent = {
  title: "Privacy Policy",
  lastUpdated: "Last updated: June 12, 2026",
  backToLogin: "Back to login",
  intro:
    "Your privacy is important to us. This Privacy Policy explains how flamingo-armond collects, uses, and protects your information.",
  sections: [
    {
      title: "1. Information We Collect",
      body: "We collect the following information when you use the Service: (a) Account information: your name, email address, and profile picture provided by Google when you sign in via Google OAuth; (b) Content: the flashcards and cardgroups you create within the Service; (c) Learning data: your swipe ratings, review history, and spaced-repetition scheduling data (FSRS); (d) Usage logs: request timestamps, IP addresses, and browser information for diagnostics and security.",
    },
    {
      title: "2. How We Use Your Information",
      body: "We use the information we collect to: (a) provide, maintain, and improve the Service; (b) synchronize your learning progress across sessions; (c) diagnose and resolve technical issues; (d) detect and prevent misuse of the Service.",
    },
    {
      title: "3. Data Storage and Security",
      body: "Your data is stored on Supabase (PostgreSQL) servers. All data is transmitted over HTTPS. We implement reasonable technical and organizational measures to protect your data against unauthorized access, alteration, or destruction. However, no method of transmission over the Internet is completely secure.",
    },
    {
      title: "4. Third-Party Services",
      body: "We use the following third-party services to operate the Service: (a) Google OAuth for authentication; (b) Supabase for database storage; (c) Vercel for hosting and content delivery. Each third party has its own privacy policy governing how they handle your data.",
    },
    {
      title: "5. Data Retention",
      body: "We retain your data for as long as your account is active. If you delete your account, we will delete your personal data and content from our systems within a reasonable period. Usage logs may be retained for a limited period for security and diagnostic purposes.",
    },
    {
      title: "6. Your Rights",
      body: "You have the right to access, correct, or delete your personal data at any time. You can delete your account and all associated data through the Service settings. For any additional data requests, please contact us via our GitHub repository.",
    },
    {
      title: "7. Children's Privacy",
      body: "The Service is not directed at children under the age of 13. We do not knowingly collect personal information from children under 13. If we become aware that we have collected such information, we will delete it promptly.",
    },
    {
      title: "8. Changes to This Policy",
      body: "We may update this Privacy Policy from time to time. We will notify you of any significant changes by posting the updated Policy on this page and updating the date at the top. Your continued use of the Service after changes constitutes your acceptance of the revised Policy.",
    },
    {
      title: "9. Contact",
      body: "If you have any questions or concerns about this Privacy Policy, please open an issue on our GitHub repository.",
    },
  ],
};

export const PRIVACY_JA: PrivacyContent = {
  title: "プライバシーポリシー",
  lastUpdated: "最終更新日：2026年6月12日",
  backToLogin: "ログインに戻る",
  intro:
    "お客様のプライバシーは私たちにとって重要です。本プライバシーポリシーでは、flamingo-armondがお客様の情報をどのように収集・利用・保護するかを説明します。",
  sections: [
    {
      title: "1. 収集する情報",
      body: "本サービスのご利用に際して、以下の情報を収集します：（a）アカウント情報：Google OAuthでログインする際にGoogleから提供される氏名・メールアドレス・プロフィール画像、（b）コンテンツ：本サービス内で作成したフラッシュカードおよびカードグループ、（c）学習データ：スワイプ評価・復習履歴・間隔反復スケジューリングデータ（FSRS）、（d）利用ログ：診断およびセキュリティのためのリクエストタイムスタンプ・IPアドレス・ブラウザ情報。",
    },
    {
      title: "2. 情報の利用目的",
      body: "収集した情報は以下の目的で利用します：（a）本サービスの提供・維持・改善、（b）学習進捗のセッション間同期、（c）技術的な問題の診断と解決、（d）本サービスの不正利用の検出と防止。",
    },
    {
      title: "3. データの保存とセキュリティ",
      body: "お客様のデータはSupabase（PostgreSQL）サーバーに保存されます。すべてのデータはHTTPSで暗号化して送信されます。不正アクセス・改ざん・破壊からデータを保護するために合理的な技術的・組織的措置を講じていますが、インターネット上の送信に完全なセキュリティはありません。",
    },
    {
      title: "4. 第三者サービス",
      body: "本サービスの運営に以下の第三者サービスを利用しています：（a）Google OAuth（認証）、（b）Supabase（データベースストレージ）、（c）Vercel（ホスティングおよびコンテンツ配信）。各第三者はそれぞれのプライバシーポリシーに基づいてデータを取り扱います。",
    },
    {
      title: "5. データの保持",
      body: "アカウントが有効な間、お客様のデータを保持します。アカウントを削除した場合、合理的な期間内に個人データおよびコンテンツをシステムから削除します。セキュリティおよび診断目的のため、利用ログは限られた期間保持される場合があります。",
    },
    {
      title: "6. お客様の権利",
      body: "お客様は、いつでも個人データへのアクセス・修正・削除を要求する権利があります。サービス設定からアカウントおよびすべての関連データを削除できます。その他のデータに関するご要望については、GitHubリポジトリのIssueよりお問い合わせください。",
    },
    {
      title: "7. 子供のプライバシー",
      body: "本サービスは13歳未満のお子様を対象としていません。13歳未満のお子様から意図的に個人情報を収集することはありません。そのような情報を収集したことが判明した場合、速やかに削除します。",
    },
    {
      title: "8. ポリシーの変更",
      body: "本プライバシーポリシーは随時更新される場合があります。重要な変更がある場合は、本ページに更新版を掲載し、上部の日付を更新することでお知らせします。変更後も本サービスを継続してご利用いただくことにより、改訂されたポリシーに同意したものとみなされます。",
    },
    {
      title: "9. お問い合わせ",
      body: "本プライバシーポリシーに関するご質問やご懸念がございましたら、GitHubリポジトリのIssueよりお問い合わせください。",
    },
  ],
};
