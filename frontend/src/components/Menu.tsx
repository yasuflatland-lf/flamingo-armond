import React from 'react';
import {Link, Route, Routes} from 'react-router-dom';
import Home from '../pages/Home';
import About from '../pages/About';
import { IoIosHome } from "react-icons/io";
import './Menu.css';

function Menu() {
    return (
        <>
            <div className="menu">
                <button className="menu-button"><Link to="/"><span className="icon-[mdi-light--home] text-4xl"> <IoIosHome /></span></Link></button>
                <button className="menu-button"><Link to="/about">About</Link></button>
            </div>
            <Routes>
                <Route path="/" element={<Home/>}/>
                <Route path="/about" element={<About/>}/>
            </Routes>

        </>
    );
}

export default Menu;