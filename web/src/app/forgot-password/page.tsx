"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ArrowLeft, CheckCircle2, LoaderCircle, MailCheck, ShieldCheck } from "lucide-react";
import { toast } from "sonner";

import { AuthBrandMark } from "@/components/auth-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { requestPasswordResetCode, resetBusinessUserPasswordByEmail } from "@/lib/api";
import { usePublicSiteSettings } from "@/lib/site-settings";

export default function ForgotPasswordPage() {
  const navigate = useNavigate();
  const site = usePublicSiteSettings();
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [cooldownLeft, setCooldownLeft] = useState(0);
  const [isSendingCode, setIsSendingCode] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);

  useEffect(() => {
    if (cooldownLeft <= 0) {
      return;
    }
    const timer = window.setInterval(() => {
      setCooldownLeft((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [cooldownLeft]);

  const handleSendCode = async () => {
    const normalizedEmail = email.trim();
    if (!normalizedEmail) {
      toast.error("请输入邮箱");
      return;
    }
    if (cooldownLeft > 0 || isSendingCode) {
      return;
    }
    setIsSendingCode(true);
    try {
      const result = await requestPasswordResetCode(normalizedEmail);
      setCooldownLeft(Math.max(1, result.cooldownSeconds || 60));
      toast.success("如果邮箱已注册，验证码会发送到该邮箱");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "发送验证码失败");
    } finally {
      setIsSendingCode(false);
    }
  };

  const handleResetPassword = async () => {
    const normalizedEmail = email.trim();
    const nextPassword = password.trim();
    if (!normalizedEmail || !code.trim() || !nextPassword || !confirmPassword.trim()) {
      toast.error("请输入邮箱、验证码和新密码");
      return;
    }
    if (nextPassword.length < 6) {
      toast.error("新密码至少 6 位");
      return;
    }
    if (nextPassword !== confirmPassword.trim()) {
      toast.error("两次输入的新密码不一致");
      return;
    }
    setIsSubmitting(true);
    try {
      await resetBusinessUserPasswordByEmail({
        email: normalizedEmail,
        code,
        password: nextPassword,
      });
      setCompleted(true);
      toast.success("密码已重置");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "重置密码失败");
    } finally {
      setIsSubmitting(false);
    }
  };

  const inputClass =
    "h-12 rounded-lg border-white/15 bg-[#182130] text-white shadow-none placeholder:text-[#7f8a9d] focus-visible:ring-[rgba(80,183,255,0.18)]";

  return (
    <div className="relative min-h-screen overflow-hidden bg-black px-4 py-6 text-white sm:px-6 lg:px-8">
      <div className="pointer-events-none absolute -left-[16vw] -top-[22%] h-[72vh] w-[62vw] rotate-[18deg] bg-[radial-gradient(ellipse_at_0%_0%,rgba(160,205,255,0.68),rgba(70,135,235,0.36)_42%,rgba(70,135,235,0)_85%)] opacity-80 blur-[64px]" />
      <div className="pointer-events-none absolute -right-[16vw] top-[-18%] h-[72vh] w-[62vw] -rotate-[18deg] bg-[radial-gradient(ellipse_at_100%_0%,rgba(160,205,255,0.68),rgba(70,135,235,0.36)_42%,rgba(70,135,235,0)_85%)] opacity-80 blur-[64px]" />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)] bg-[size:48px_48px] opacity-20 [mask-image:radial-gradient(ellipse_at_50%_42%,#000_0%,transparent_72%)]" />

      <header className="relative z-10 mx-auto flex max-w-[1200px] items-center justify-between">
        <Link to="/login" className="inline-flex items-center gap-3 text-sm font-semibold text-[#c8d1df] transition hover:text-white">
          <ArrowLeft className="size-4" />
          返回登录
        </Link>
        <div className="inline-flex items-center gap-3 text-base font-bold">
          <AuthBrandMark logoUrl={site.logoUrl} siteName={site.name} />
          <span>{site.name || "Image Studio"}</span>
        </div>
      </header>

      <main className="relative z-10 mx-auto flex min-h-[calc(100vh-72px)] w-full max-w-[520px] items-center py-10">
        <section className="relative w-full overflow-hidden rounded-lg border border-white/15 bg-[#05070c] p-6 shadow-[0_34px_120px_rgba(0,0,0,0.74),0_0_74px_rgba(44,137,255,0.16),inset_0_1px_0_rgba(255,255,255,0.16)] sm:p-8">
          <div className="pointer-events-none absolute inset-x-0 top-0 h-28 bg-[radial-gradient(circle_at_30%_0%,rgba(91,214,255,0.16),transparent_18rem),linear-gradient(180deg,rgba(255,255,255,0.065),transparent)]" />
          <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.035),transparent_42%)]" />
          <div className="relative z-10">
            <div className="inline-flex size-12 items-center justify-center rounded-lg border border-cyan-200/30 bg-cyan-200/10 text-cyan-100">
              {completed ? <CheckCircle2 className="size-6" /> : <ShieldCheck className="size-6" />}
            </div>
            <h1 className="mt-6 text-3xl font-bold leading-tight text-white">
              {completed ? "密码已重置" : "重置密码"}
            </h1>
            <p className="mt-3 text-sm leading-6 text-[#9ca6b8]">
              {completed ? "请使用新密码重新登录。" : "通过注册邮箱接收验证码，然后设置新密码。"}
            </p>

            {completed ? (
              <Button
                type="button"
                className="mt-8 h-12 w-full rounded-lg border border-cyan-200/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.18),rgba(255,255,255,0.05)),linear-gradient(135deg,rgba(63,105,255,0.9),rgba(31,220,255,0.78))] text-white shadow-[0_18px_40px_rgba(41,152,255,0.26)]"
                onClick={() => navigate("/login", { replace: true })}
              >
                返回登录
              </Button>
            ) : (
              <div className="mt-7 grid gap-4">
                <Field label="邮箱" htmlFor="reset-email">
                  <div className="flex gap-2">
                    <Input
                      id="reset-email"
                      name="email"
                      type="email"
                      autoComplete="email"
                      value={email}
                      onChange={(event) => setEmail(event.target.value)}
                      placeholder="you@example.com"
                      className={inputClass}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      className="h-12 min-w-[96px] shrink-0 rounded-lg border-white/15 bg-[#111823] text-[#e9eef7] shadow-none hover:bg-[#182130]"
                      onClick={() => void handleSendCode()}
                      disabled={cooldownLeft > 0 || isSendingCode || isSubmitting}
                    >
                      {isSendingCode ? <LoaderCircle className="size-4 animate-spin" /> : <MailCheck className="size-4" />}
                      {cooldownLeft > 0 ? `${cooldownLeft}s` : "发送"}
                    </Button>
                  </div>
                </Field>

                <Field label="验证码" htmlFor="reset-code">
                  <Input
                    id="reset-code"
                    name="code"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={code}
                    onChange={(event) => setCode(event.target.value)}
                    placeholder="邮箱验证码"
                    className={inputClass}
                  />
                </Field>

                <Field label="新密码" htmlFor="reset-password">
                  <Input
                    id="reset-password"
                    name="password"
                    type="password"
                    autoComplete="new-password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    placeholder="至少 6 位"
                    className={inputClass}
                  />
                </Field>

                <Field label="确认新密码" htmlFor="reset-confirm-password">
                  <Input
                    id="reset-confirm-password"
                    name="confirm-password"
                    type="password"
                    autoComplete="new-password"
                    value={confirmPassword}
                    onChange={(event) => setConfirmPassword(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        void handleResetPassword();
                      }
                    }}
                    placeholder="请再次输入新密码"
                    className={inputClass}
                  />
                </Field>

                <Button
                  type="button"
                  className="mt-2 h-12 w-full rounded-lg border border-cyan-200/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.18),rgba(255,255,255,0.05)),linear-gradient(135deg,rgba(63,105,255,0.9),rgba(31,220,255,0.78))] text-white shadow-[0_18px_40px_rgba(41,152,255,0.26)] hover:bg-[linear-gradient(135deg,rgba(255,255,255,0.22),rgba(255,255,255,0.08)),linear-gradient(135deg,rgba(63,105,255,0.95),rgba(31,220,255,0.82))]"
                  onClick={() => void handleResetPassword()}
                  disabled={isSubmitting || isSendingCode}
                >
                  {isSubmitting ? <LoaderCircle className="size-4 animate-spin" /> : <ShieldCheck className="size-4" />}
                  重置密码
                </Button>
              </div>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: ReactNode;
}) {
  return (
    <div className="grid gap-2">
      <label htmlFor={htmlFor} className="text-sm font-semibold text-[#c6cfdd]">
        {label}
      </label>
      {children}
    </div>
  );
}
