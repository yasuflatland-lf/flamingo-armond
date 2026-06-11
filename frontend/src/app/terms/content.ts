type Section = { title: string; body: string };

type TermsContent = {
  title: string;
  lastUpdated: string;
  backToLogin: string;
  sections: Section[];
};

export const TERMS_EN: TermsContent = {
  title: "Terms of Service",
  lastUpdated: "Last updated: June 12, 2026",
  backToLogin: "Back to login",
  sections: [
    {
      title: "1. Acceptance of Terms",
      body: 'By accessing or using flamingo-armond (the "Service"), you agree to be bound by these Terms of Service. If you do not agree to these terms, please do not use the Service.',
    },
    {
      title: "2. Description of Service",
      body: "flamingo-armond is a personal flashcard application that uses spaced repetition to help you study and memorize content. The Service is intended for personal and educational use.",
    },
    {
      title: "3. Account Registration",
      body: "You must sign in with a Google account to use the Service. You are responsible for all activity that occurs under your account. Please notify us immediately if you suspect unauthorized use of your account.",
    },
    {
      title: "4. Acceptable Use",
      body: "You may use the Service only for lawful, personal, and educational purposes. You agree not to: (a) use the Service for any illegal or unauthorized purpose; (b) attempt to gain unauthorized access to any part of the Service or its infrastructure; (c) interfere with or disrupt the integrity or performance of the Service; or (d) create or store content that is harmful, offensive, or infringes on the rights of others.",
    },
    {
      title: "5. User Content",
      body: "You retain ownership of the flashcards and other content you create in the Service. By using the Service, you grant us a limited license to store, process, and display your content solely for the purpose of providing the Service to you. You are solely responsible for the content you create.",
    },
    {
      title: "6. Service Availability",
      body: 'We provide the Service on an "as is" and "as available" basis. We do not guarantee uninterrupted, timely, or error-free access. We reserve the right to modify, suspend, or discontinue any part of the Service at any time without prior notice.',
    },
    {
      title: "7. Termination",
      body: "You may stop using the Service and request deletion of your account at any time. We reserve the right to suspend or terminate your access to the Service at our discretion, without notice, for conduct that we determine violates these Terms or is harmful to other users, us, or third parties.",
    },
    {
      title: "8. Disclaimer of Warranties",
      body: 'THE SERVICE IS PROVIDED "AS IS" WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, OR NON-INFRINGEMENT. WE DO NOT WARRANT THAT THE SERVICE WILL BE UNINTERRUPTED, SECURE, OR ERROR-FREE.',
    },
    {
      title: "9. Limitation of Liability",
      body: "TO THE MAXIMUM EXTENT PERMITTED BY APPLICABLE LAW, WE SHALL NOT BE LIABLE FOR ANY INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL, OR PUNITIVE DAMAGES, OR ANY LOSS OF PROFITS, DATA, OR GOODWILL, ARISING OUT OF OR IN CONNECTION WITH YOUR USE OF THE SERVICE.",
    },
    {
      title: "10. Changes to These Terms",
      body: "We may update these Terms of Service from time to time. The updated Terms will be effective upon posting to the Service. Your continued use of the Service after any changes constitutes your acceptance of the revised Terms. We will indicate the date of the latest revision at the top of this page.",
    },
    {
      title: "11. Contact",
      body: "If you have any questions about these Terms of Service, please open an issue on our GitHub repository.",
    },
  ],
};

export const TERMS_JA: TermsContent = {
  title: "利用規約",
  lastUpdated: "最終更新日：2026年6月12日",
  backToLogin: "ログインに戻る",
  sections: [
    {
      title: "1. 利用規約への同意",
      body: "flamingo-armond（以下「本サービス」）にアクセスまたはご利用いただくことにより、本利用規約に同意したものとみなされます。同意いただけない場合は、本サービスのご利用はお控えください。",
    },
    {
      title: "2. サービスの説明",
      body: "flamingo-armondは、間隔反復法を用いた個人向けフラッシュカードアプリです。学習・暗記のサポートを目的として提供され、個人的・教育的な用途を想定しています。",
    },
    {
      title: "3. アカウント登録",
      body: "本サービスのご利用にはGoogleアカウントによるログインが必要です。お客様のアカウントで発生したすべての活動についてお客様が責任を負います。アカウントへの不正アクセスが疑われる場合は、直ちにご連絡ください。",
    },
    {
      title: "4. 禁止事項",
      body: "本サービスは、合法的かつ個人的・教育的な目的においてのみご利用いただけます。以下の行為を禁止します：（a）違法または不正な目的での利用、（b）本サービスまたはそのインフラへの不正アクセスの試み、（c）本サービスの整合性またはパフォーマンスへの妨害、（d）有害・不快なコンテンツや第三者の権利を侵害するコンテンツの作成・保存。",
    },
    {
      title: "5. ユーザーコンテンツ",
      body: "本サービスで作成したフラッシュカードおよびその他コンテンツの所有権はお客様に帰属します。本サービスのご利用により、本サービス提供のみを目的として、お客様のコンテンツを保存・処理・表示する限定的なライセンスを当社に付与するものとします。作成したコンテンツに関する責任はお客様が負います。",
    },
    {
      title: "6. サービスの提供",
      body: "本サービスは「現状有姿」かつ「利用可能な状態」で提供されます。継続的・タイムリー・エラーなしのアクセスは保証しません。事前の通知なく、本サービスの全部または一部を変更・中断・廃止する権利を留保します。",
    },
    {
      title: "7. 利用の終了",
      body: "いつでも本サービスのご利用を停止し、アカウントの削除を申請することができます。当社は、本利用規約に違反する、またはユーザー・当社・第三者に有害と判断する行為に対して、通知なしにお客様のアクセスを停止または終了する権利を留保します。",
    },
    {
      title: "8. 保証の否認",
      body: "本サービスは、商品性・特定目的への適合性・非侵害性を含む明示または黙示の一切の保証なしに「現状有姿」で提供されます。本サービスが中断なく、安全に、エラーなく提供されることは保証しません。",
    },
    {
      title: "9. 責任の制限",
      body: "適用法の最大限許容される範囲において、当社は本サービスのご利用に起因または関連する、利益・データ・のれんの損失を含む間接的・付随的・特別・結果的・懲罰的損害賠償について責任を負いません。",
    },
    {
      title: "10. 利用規約の変更",
      body: "本利用規約は随時更新される場合があります。更新された規約は本サービスに掲載された時点で有効となります。変更後も本サービスを継続してご利用いただくことにより、改訂された規約に同意したものとみなされます。最新の改訂日はページ上部に表示されます。",
    },
    {
      title: "11. お問い合わせ",
      body: "本利用規約に関するご質問は、GitHubリポジトリのIssueよりお問い合わせください。",
    },
  ],
};
