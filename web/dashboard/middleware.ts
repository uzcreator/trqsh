import { NextResponse, type NextRequest } from "next/server";

// Route protection: without a session cookie, everything except /login redirects
// to /login; with one, /login redirects into the app. JWT validity is enforced
// by the API (a 401 in a page triggers a redirect to /login there).
export function middleware(req: NextRequest) {
  const hasSession = req.cookies.has("trqsh_access") || req.cookies.has("trqsh_refresh");
  const { pathname } = req.nextUrl;
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
