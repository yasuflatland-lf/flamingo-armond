import React from "react";
import { Link } from "react-router-dom";
import { IoIosSettings } from "react-icons/io";

function Settings() {
  return (
    <div className="bg-white text-black">
      <header className="flex items-center justify-between px-4 py-2 border-b border-gray-300">
        <button className="text-blue-500">
          {"<"} <Link to="/">Back</Link>
        </button>
        <h1 className="text-lg font-semibold">Settings</h1>
        <div className="w-8"></div>
      </header>
      <div className="p-4">
        <section>
          <h2 className="text-sm font-semibold text-gray-500">General</h2>
          <ul className="mt-2 space-y-4">
            <li className="flex items-center justify-between">
              <span>Theme</span>
              <span className="text-gray-500">Light Mode</span>
            </li>
            <li className="flex items-center justify-between">
              <span>Language</span>
              <span className="text-gray-500">English</span>
            </li>
            <li className="flex items-center justify-between">
              <span>Notifications</span>
              <input type="checkbox" className="toggle-checkbox" />
            </li>
            <li className="flex items-center justify-between">
              <span>Location</span>
              <input type="checkbox" className="toggle-checkbox" />
            </li>
          </ul>
        </section>
        <section className="mt-6">
          <h2 className="text-sm font-semibold text-gray-500">
            Account & Security
          </h2>
          <ul className="mt-2 space-y-4">
            <li className="flex items-center justify-between">
              <span>Account Information</span>
              <span>{">"}</span>
            </li>
            <li className="flex items-center justify-between">
              <span>Security & Authentications</span>
              <span>{">"}</span>
            </li>
          </ul>
        </section>
        <section className="mt-6">
          <h2 className="text-sm font-semibold text-gray-500">Other</h2>
          <ul className="mt-2 space-y-4">
            <li className="flex items-center justify-between">
              <span>Privacy Policy</span>
              <span>{">"}</span>
            </li>
            <li className="flex items-center justify-between">
              <span>Terms & Conditions</span>
              <span>{">"}</span>
            </li>
            <li className="flex items-center justify-between">
              <span>About Us</span>
              <span>{">"}</span>
            </li>
          </ul>
        </section>
        <section className="mt-6">
          <h2 className="text-sm font-semibold text-gray-500">App Version</h2>
          <p className="mt-2">1.0.0</p>
        </section>
      </div>
    </div>
  );
}

export default Settings;
