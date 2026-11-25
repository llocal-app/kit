import { Switch, Route } from "wouter";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "./lib/queryClient";
import Landing from "@/pages/landing";
import Tools from "@/pages/tools";
import Tool from "@/pages/tool";

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <Switch>
        <Route path="/" component={Landing} />
        <Route path="/tools" component={Tools} />
        <Route path="/tools/:id" component={Tool} />
      </Switch>
    </QueryClientProvider>
  );
}
