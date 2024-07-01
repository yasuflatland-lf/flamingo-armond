import React from "react";
import { FaGoogle } from "react-icons/fa";
import { Link } from "react-router-dom";
import { IoIosHome } from "react-icons/io";

function Login() {
  return (
    <div className="flex items-center justify-center min-h-screen">
      <form className="bg-white p-6 rounded-lg w-full max-w-sm">
        <h2 className="text-center text-2xl mb-6">Login</h2>
        <div className="mb-4 text-center">
          <button
            type="submit"
            className="w-full bg-pink-700 text-white py-2 px-4 rounded-md hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 flex items-center justify-center"
          >
            <FaGoogle className="mr-2" />
            <Link to="/">Login with Google</Link>
          </button>
        </div>
      </form>
    </div>
  );
}

export default Login;
