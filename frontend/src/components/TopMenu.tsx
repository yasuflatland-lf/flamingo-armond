import React from 'react';
import './TopMenu.css';
import {IoIosSettings} from "react-icons/io";

function TopMenu() {
    return (
        <div className="top-menu">
            <button className="menu-button">
                <span className="icon-[mdi-light--home] text-2xl"> <IoIosSettings/></span>
            </button>
        </div>
    );
}

export default TopMenu;
