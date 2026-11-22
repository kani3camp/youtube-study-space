import { Link } from '@tanstack/react-router'
import { ExternalLink, guideURL } from './features/mypage/view'
import { publicPolicyConfig } from './public-policy-config'

function ContactDestination({ href }: { href?: string }) {
	if (!href) return null
	return (
		<div className="contact-destination">
			<ExternalLink href={href} primary>
				お問い合わせフォームを開く
			</ExternalLink>
		</div>
	)
}

export function ContactPage({
	supportError = false,
	proofRef = null,
}: {
	supportError?: boolean
	proofRef?: string | null
}) {
	const config = publicPolicyConfig()
	return (
		<main className="main-content public-page contact-page">
			<Link className="contact-back" to="/">
				トップへ戻る
			</Link>
			<header className="contact-heading">
				<h1>お問い合わせ</h1>
				<p>
					ご質問やお困りごと、保存データに関するご依頼はこちらから。
					マイページにログインできない場合も利用できる窓口をご案内します。
				</p>
			</header>
			{supportError && (
				<p className="notice" role="alert">
					YouTubeでの本人確認を完了できませんでした。お手数ですが、受付窓口からもう一度お手続きください。
				</p>
			)}
			{proofRef && (
				<section className="card" aria-labelledby="support-complete">
					<h2 id="support-complete">依頼の本人確認が完了しました</h2>
					<p>
						受付窓口で依頼内容と照合して対応します。データの削除やログインの解除は、この画面では実行していません。
					</p>
					<p>確認結果の参照番号</p>
					<p className="proof-reference">{proofRef}</p>
					<p>
						この番号は、画面を離れるか再読み込みすると表示されなくなります。確認結果は受付窓口からご案内します。
					</p>
				</section>
			)}
			<div className="contact-options">
				<section
					className="card contact-option"
					aria-labelledby="contact-general"
				>
					<p className="contact-eyebrow">使い方・不具合・ご意見</p>
					<h2 id="contact-general">一般のお問い合わせ</h2>
					<p>使い方のご質問、不具合の報告、機能のご要望などを受け付けます。</p>
					<h3>不具合の報告でお知らせいただきたいこと</h3>
					<ul>
						<li>発生した日時と、問題が起きた画面</li>
						<li>行った操作と、表示されたエラーメッセージ</li>
						<li>利用している端末とブラウザ</li>
					</ul>
					<p className="muted">
						画像を添付する場合は、個人情報や他の方の情報が写らないようにしてください。
					</p>
					<ContactDestination href={config.contactURL} />
				</section>
				<section
					className="card contact-option"
					aria-labelledby="contact-privacy"
				>
					<p className="contact-eyebrow">ご自身のデータ・アカウント</p>
					<h2 id="contact-privacy">個人情報・プライバシーのご相談</h2>
					<p>
						Google連携解除後のご連絡、保存データの削除・開示・訂正、すべてのログインの解除、個人情報の取り扱いに関するご相談を受け付けます。Google連携を解除済みでマイページにログインできない場合も、こちらからご連絡いただけます。
					</p>
					<h3>連絡時にお知らせいただきたいこと</h3>
					<ul>
						<li>希望する手続きや、ご相談の内容</li>
						<li>回答を受け取れる連絡先</li>
						<li>分かる場合は、対象のYouTubeチャンネルのURL</li>
					</ul>
					<p className="muted">
						使い方の確認やマイページへのログインを、ご依頼の条件にはしません。
					</p>
					<ContactDestination href={config.contactURL} />
				</section>
			</div>
			<section
				className="card contact-process"
				aria-labelledby="contact-process"
			>
				<h2 id="contact-process">データに関するご依頼の流れ</h2>
				<ol>
					<li>
						<strong>お問い合わせ窓口へ連絡</strong>
						<p>ご依頼を受け付けた後、必要な手続きを返信でご案内します。</p>
					</li>
					<li>
						<strong>依頼の対象チャンネルの本人確認</strong>
						<p>
							ご依頼の受付後は、原則として通常のログインとは別のGoogle認証手続きで、対象のYouTubeチャンネルを確認します。Google連携を解除済みなどで認証手続きを利用できない場合も、まずはお問い合わせ窓口へご連絡ください。個別の確認方法をご案内します。
						</p>
					</li>
					<li>
						<strong>内容の確認・対応と結果のご連絡</strong>
						<p>
							本人確認の完了だけで削除や全ログインの解除が実行されることはありません。対応結果は、受付時の連絡先へお知らせします。
						</p>
					</li>
				</ol>
				<p className="contact-safety">
					パスワード・認証コード・本人確認リンクは、問い合わせ本文や公開のライブチャットへ載せないでください。
				</p>
			</section>
			<aside className="contact-help" aria-label="関連するご案内">
				<p>
					操作方法を調べたいときは、
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
					もご利用いただけます。
				</p>
				<Link to="/privacy">プライバシーポリシー</Link>
				<p>運営者・サービスの公開表記：{config.operatorName}</p>
			</aside>
		</main>
	)
}
