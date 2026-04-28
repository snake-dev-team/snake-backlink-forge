import { Suspense } from "react";
import { LoginForm } from "./login-form";

export default function LoginPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-12 text-foreground">
      <Suspense fallback={null}>
        <LoginForm />
      </Suspense>
    </main>
  );
}
