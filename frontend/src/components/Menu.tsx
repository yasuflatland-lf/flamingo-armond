import React from "react";
import {Link, Route, Routes} from "react-router-dom";
import Home from "../pages/Home";
import About from "../pages/About";
import {IoIosHome} from "react-icons/io";

function Menu() {
    return (
        <>
            <div className="fixed bottom-0 w-full bg-pink-700 flex justify-around py-5">
                <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
                    <Link to="/">
                        <span className="icon-[mdi-light--home] text-3xl">
                          <IoIosHome/>
                        </span>
                    </Link>
                </button>
                <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
                    <Link to="/about">About</Link>
                </button>
            </div>
            <Routes>
                <Route path="/" element={<Home/>}/>
                <Route path="/about" element={<About/>}/>
            </Routes>
        </>
    );
}

export default Menu;
