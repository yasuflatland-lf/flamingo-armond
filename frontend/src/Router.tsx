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

// Mocking the isLogin variable for the sake of this example
const isLogin = true; // This should be replaced with your actual login check logic

const PrivateRoutes = () => {
  const location = useLocation();

  // 未ログインチェック
  // （isLoginを判定する処理は省略してます）
  if (isLogin === false) {
    return <Navigate to="/login" state={{ redirectPath: location.pathname }} />;
  }
  return <Outlet />;
};

function Router() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="login" element={<Login />} />
        {/* PrivateRoutes内に設定された画面はサインインが必須になる */}
        <Route element={<PrivateRoutes />}>
          <Route path="/" element={<Home />} />
        </Route>
        <Route path="/settings" element={<Settings />} />
        <Route path="*" element={<Error404 />} />
      </Routes>
    </BrowserRouter>
  );
}

export default Router;
