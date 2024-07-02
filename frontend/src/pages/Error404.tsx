import React from "react";
import { Link } from "react-router-dom";

const Error404 = () => {
  return (
    <div className="bg-pink-700 flex flex-col items-center justify-center h-screen text-gray-100 text-center p-8">
      <h1 className="text-6xl font-bold mb-4">404 - Page Not Found</h1>
      <p className="text-2xl mb-6">
        Sorry, the page you are looking for does not exist.
      </p>
      <Link
        to="/"
        className="bg-white text-pink-700 font-black text-lg rounded-full px-6 py-3 hover:bg-gray-200"
      >
        Go to Home
      </Link>
    </div>
  );
};

export default Error404;
