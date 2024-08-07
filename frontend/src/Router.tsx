// Router.tsx
import React from "react";
import {
  BrowserRouter,
  Navigate,
  Outlet,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import Login from "./pages/Login"; // Import your SignIn component
import Home from "./pages/Home"; // Import your Home component
import Error404 from "./pages/Error404";
import Settings from "./pages/Settings";
import WordList from "./pages/WordList";
import LoadingPage from "./components/LoadingPage.tsx";

// Mocking the isLogin variable for the sake of this example
// This should be replaced with your actual login check logic
const isLogin = true;

const PrivateRoutes = () => {
  const location = useLocation();

  // Unauthenticated check
  // (The process for determining isLogin is omitted)
  if (!isLogin) {
    return <Navigate to="/login" state={{ redirectPath: location.pathname }} />;
  }
  return <Outlet />;
};

function Router() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="login" element={<Login />} />
        {/* Screens set within PrivateRoutes require sign-in */}
        <Route element={<PrivateRoutes />}>
          <Route path="/" element={<Home />} />
        </Route>
        <Route path="/settings" element={<Settings />} />
        <Route path="/list" element={<WordList />} />
        <Route path="/ldtest" element={<LoadingPage />} />
        <Route path="*" element={<Error404 />} />
      </Routes>
    </BrowserRouter>
  );
}

export default Router;
