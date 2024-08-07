import React, { useState } from "react";
import SwipeableCard from "../components/SwipeableCard.tsx";
import TopMenu from "../components/TopMenu.tsx";
import Menu from "../components/Menu.tsx";

function Home() {
  const [cards] = useState([
    { id: 1, content: "Brown Fox Leap Over The Box", subtitle: "sub 1" },
    { id: 2, content: "Card 2", subtitle: "sub 2" },
    { id: 3, content: "Card 3", subtitle: "sub 3" },
  ]);

  const [currentCardIndex, setCurrentCardIndex] = useState(0);
  const likeMenu = true;

  const handleSwiped = () => {
    setCurrentCardIndex((prev) => (prev + 1) % cards.length);
  };

  return (
    <>
      <TopMenu />
      {cards.length > 0 && (
        <SwipeableCard
          key={cards[currentCardIndex].id}
          content={cards[currentCardIndex].content}
          subtitle={cards[currentCardIndex].subtitle}
          onSwiped={handleSwiped}
        />
      )}
      <Menu  likeMenu={likeMenu}/>
    </>
  );
}

export default Home;
