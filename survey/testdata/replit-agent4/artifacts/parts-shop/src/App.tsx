import { ClerkProvider, SignedIn, SignedOut, SignIn } from "@clerk/react";
import { Route, Switch } from "wouter";
import Orders from "./pages/Orders";
import Order from "./pages/Order";

export default function App() {
  return (
    <ClerkProvider publishableKey={import.meta.env.VITE_CLERK_PUBLISHABLE_KEY}>
      <SignedOut><SignIn /></SignedOut>
      <SignedIn>
        <Switch>
          <Route path="/" component={Orders} />
          <Route path="/orders/:id" component={Order} />
        </Switch>
      </SignedIn>
    </ClerkProvider>
  );
}
