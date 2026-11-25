import * as client from "openid-client";
import { Strategy, type VerifyFunction } from "openid-client/passport";
import passport from "passport";
import session from "express-session";
import type { Express, RequestHandler } from "express";
import memoize from "memoizee";
import connectPg from "connect-pg-simple";

if (!process.env.REPLIT_DOMAINS) {
  throw new Error("Environment variable REPLIT_DOMAINS not provided");
}

const getOidcConfig = memoize(
  async () => client.discovery(new URL(process.env.ISSUER_URL ?? "https://replit.com/oidc"), process.env.REPL_ID!),
  { maxAge: 3600 * 1000 },
);

export async function setupAuth(app: Express) {
  const pgStore = connectPg(session);
  app.set("trust proxy", 1);
  app.use(session({
    secret: process.env.SESSION_SECRET!,
    store: new pgStore({ conString: process.env.DATABASE_URL, tableName: "sessions" }),
    resave: false,
    saveUninitialized: false,
  }));
  app.use(passport.initialize());
  app.use(passport.session());

  const config = await getOidcConfig();
  const verify: VerifyFunction = (tokens, done) => done(null, { claims: tokens.claims() });
  for (const domain of process.env.REPLIT_DOMAINS!.split(",")) {
    passport.use(new Strategy({ name: `replitauth:${domain}`, config, scope: "openid email profile offline_access",
      callbackURL: `https://${domain}/api/callback` }, verify));
  }

  app.get("/api/login", (req, res, next) => passport.authenticate(`replitauth:${req.hostname}`)(req, res, next));
  app.get("/api/callback", (req, res, next) =>
    passport.authenticate(`replitauth:${req.hostname}`, { successReturnToOrRedirect: "/", failureRedirect: "/api/login" })(req, res, next));
  app.get("/api/logout", (req, res) => req.logout(() => res.redirect("/")));
}

export const isAuthenticated: RequestHandler = (req, res, next) => (req.isAuthenticated() ? next() : res.status(401).json({ message: "Unauthorized" }));
