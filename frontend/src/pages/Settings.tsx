import React from "react";
import { Link } from "react-router-dom";
import { IoIosSettings } from "react-icons/io";

function Settings() {
  return (
    <div className="bg-white text-black">
      <header className="flex items-center justify-between px-4 py-2 border-b border-gray-300 bg-pink-700">
        <button className="text-white">
          {"<"} <Link to="/">Back</Link>
        </button>
        <h1 className="text-lg font-semibold text-white">Settings</h1>
        <div className="w-8"></div>
      </header>

      <h1 className="py-2 pl-4 bg-pink-700 font-semibold text-white">
        General
      </h1>
      <section className="p-4">
        <ul className="space-y-4">
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
      <h1 className="py-2 pl-4 bg-pink-700 font-semibold text-white">
        Account & Security
      </h1>
      <section className="p-4">
        <ul className="space-y-4">
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
      <h1 className="py-2 pl-4 bg-pink-700 font-semibold text-white">Other</h1>
      <section className="p-4">
        <ul className="space-y-4">
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
      <h1 className="py-2 pl-4 bg-pink-700 font-semibold text-white">
        App Version
      </h1>
      <section className="p-4">
        <p className="">1.0.0</p>
      </section>
    </div>
  );
}

export default Settings;
