<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:choose>
			<xsl:when test="/root/item/@lang = 'fr'">
				<item>french</item>
			</xsl:when>
			<xsl:when test="/root/item/@lang = 'en'">
				<item>english</item>
			</xsl:when>
			<xsl:otherwise>
				<item>other</item>
			</xsl:otherwise>
		</xsl:choose>
	</xsl:template>
</xsl:stylesheet>