import { NextResponse, type NextRequest } from "next/server";

// Route protection: without a session cookie, everything except /login redirects
// to /login; with one, /login redirects into the app. JWT validity is enforced
// by the API (a 401 in a page triggers a redirect to /login there).
export function middleware(req: NextRequest) {
  const { pathname, hostname } = req.nextUrl;

  // qr.<base>: the /qr remote-control pairing viewer — a reserved host the edge
  // routes to this same deployment (see internal/server's reservedUpstream),
  // told apart from app.<base> by Host header. Its bare paths rewrite to
  // /remote/... internally, so the pairing code is the whole URL
  // ("qr.trqsh.uz/CODE"), matching what the CLI's QR actually encodes.
  if (
    hostname.startsWith("qr.") &&
    !pathname.startsWith("/api/") &&
    !pathname.startsWith("/_next/") &&
    !pathname.startsWith("/remote")
  ) {
    const url = req.nextUrl.clone();
    url.pathname = `/remote${pathname === "/" ? "" : pathname}`;
    return NextResponse.rewrite(url);
  }

  // /remote (the page) and /api/remote (its same-origin proxy to the control
  // plane — see app/api/remote) are always public: the session code in the URL
  // is the capability, not a login. Requiring one would defeat the point of
  // scanning a QR (see internal/api/remote.go).
  if (pathname === "/remote" || pathname.startsWith("/remote/") || pathname.startsWith("/api/remote/")) {
    return NextResponse.next();
  }

  const hasSession = req.cookies.has("trqsh_access") || req.cookies.has("trqsh_refresh");
  const isPublic = pathname === "/login" || pathname.startsWith("/login/");

  // The desktop device-approval page must be reachable in both states: signed-in
  // users approve immediately; signed-out users see a sign-in prompt that returns
  // here (never redirect it away, or the ?code= is lost).
  if (pathname === "/device" || pathname.startsWith("/device/")) {
    return NextResponse.next();
  }

  if (!hasSession && !isPublic) {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    return NextResponse.redirect(url);
  }
  if (hasSession && isPublic) {
    const url = req.nextUrl.clone();
    url.pathname = "/";
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|ico)$).*)"],
};
